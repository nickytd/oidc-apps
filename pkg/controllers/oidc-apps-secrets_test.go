// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
)

func resolvedConfigForTest(object *appsv1.Deployment) *configuration.ResolvedConfig {
	cfg := &configuration.OIDCAppsControllerConfig{
		Global: configuration.Global{
			DomainName: "example.org",
			Oauth2Proxy: &configuration.Oauth2ProxyConfig{
				ClientID:      "client-id",
				OidcIssuerURL: "https://oidc-provider.org",
			},
		},
		Targets: []configuration.Target{
			{
				Name: object.GetName(),
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: object.GetLabels(),
				},
			},
		},
	}

	return cfg.Resolve(context.Background(), object)
}

func TestMutateOauth2SecretGeneratesCookieSecretWhenAbsent(t *testing.T) {
	object := &appsv1.Deployment{
		Name:      "nginx",
		Namespace: "nginx",
		Labels:    map[string]string{"app": "nginx"},
		UID:       "deployment-uid-1",
	}

	secret := &corev1.Secret{}
	rc := resolvedConfigForTest(object)

	require.NoError(t, mutateOauth2Secret(secret, rc))

	cookieSecret, ok := secret.Data[constants.Oauth2CookieSecretKey]
	require.True(t, ok, "oauth2 secret must contain the cookie-secret key")
	assert.Len(t, cookieSecret, 32, "cookie secret must be 32 bytes")
}

func TestMutateOauth2SecretPreservesExistingCookieSecret(t *testing.T) {
	object := &appsv1.Deployment{
		Name:      "nginx",
		Namespace: "nginx",
		Labels:    map[string]string{"app": "nginx"},
		UID:       "deployment-uid-1",
	}

	existing := []byte("an-existing-cookie-secret-value0")
	secret := &corev1.Secret{
		Data: map[string][]byte{constants.Oauth2CookieSecretKey: existing},
	}
	rc := resolvedConfigForTest(object)

	require.NoError(t, mutateOauth2Secret(secret, rc))

	assert.Equal(t, existing, secret.Data[constants.Oauth2CookieSecretKey],
		"an already-persisted cookie secret must be preserved across reconciles")
}

func TestMutateOauth2SecretStableAcrossRepeatedReconciles(t *testing.T) {
	object := &appsv1.Deployment{
		Name:      "nginx",
		Namespace: "nginx",
		Labels:    map[string]string{"app": "nginx"},
		UID:       "deployment-uid-1",
	}

	secret := &corev1.Secret{}
	rc := resolvedConfigForTest(object)

	require.NoError(t, mutateOauth2Secret(secret, rc))
	first := append([]byte(nil), secret.Data[constants.Oauth2CookieSecretKey]...)

	// Subsequent reconciles run the mutate fn against the fetched secret.
	for range 3 {
		require.NoError(t, mutateOauth2Secret(secret, rc))
		assert.Equal(t, first, secret.Data[constants.Oauth2CookieSecretKey],
			"cookie secret must stay stable across reconciles (VPA in-place safety)")
	}
}

func TestMutateOauth2SecretRandomPerWorkload(t *testing.T) {
	objectA := &appsv1.Deployment{
		Name: "nginx", Namespace: "nginx",
		Labels: map[string]string{"app": "nginx"}, UID: "uid-a",
	}
	objectB := &appsv1.Deployment{
		Name: "nginx", Namespace: "nginx",
		Labels: map[string]string{"app": "nginx"}, UID: "uid-b",
	}

	secretA := &corev1.Secret{}
	secretB := &corev1.Secret{}

	require.NoError(t, mutateOauth2Secret(secretA, resolvedConfigForTest(objectA)))
	require.NoError(t, mutateOauth2Secret(secretB, resolvedConfigForTest(objectB)))

	assert.NotEqual(t, secretA.Data[constants.Oauth2CookieSecretKey], secretB.Data[constants.Oauth2CookieSecretKey],
		"independently generated cookie secrets must not collide (not derived from identity)")
}
