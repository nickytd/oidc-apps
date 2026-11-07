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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
)

// sourceRegistrySecret is the secret the controller propagates from its own namespace.
func sourceRegistrySecret() *corev1.Secret {
	return &corev1.Secret{
		Name:      testRegistrySecret,
		Namespace: testControllerNS,
		Type:      corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			DOCKERCONFIGJSON: []byte(`{"auths":{}}`),
		},
	}
}

// matchingTargetConfig matches Deployments/StatefulSets carrying app=nginx in any namespace.
func matchingTargetConfig() *configuration.OIDCAppsControllerConfig {
	return &configuration.OIDCAppsControllerConfig{
		Targets: []configuration.Target{
			{
				Name: "nginx",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "nginx"},
				},
			},
		},
	}
}

func targetDeployment(name, namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		Name:      name,
		Namespace: namespace,
		Labels:    map[string]string{"app": "nginx"},
	}
}

func targetStatefulSet(name, namespace string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		Name:      name,
		Namespace: namespace,
		Labels:    map[string]string{"app": "nginx"},
	}
}

func registrySecretExists(t *testing.T, c client.Client, namespace string) bool {
	t.Helper()

	err := c.Get(context.Background(),
		types.NamespacedName{Namespace: namespace, Name: testRegistrySecret}, &corev1.Secret{})
	if err == nil {
		return true
	}

	require.True(t, apierrors.IsNotFound(err), "unexpected error getting registry secret: %v", err)

	return false
}

func TestEnsureRegistrySecret(t *testing.T) {
	t.Run("creates a labeled copy in the target namespace", func(t *testing.T) {
		c := newOwnedResourceClient(t, sourceRegistrySecret())

		require.NoError(t, ensureRegistrySecret(context.Background(), c,
			testRegistrySecret, testControllerNS, testTargetNS))

		copied := &corev1.Secret{}
		require.NoError(t, c.Get(context.Background(),
			types.NamespacedName{Namespace: testTargetNS, Name: testRegistrySecret}, copied))

		assert.Equal(t, []byte(`{"auths":{}}`), copied.Data[DOCKERCONFIGJSON],
			"the copy must carry the source credentials")
		assert.Equal(t, constants.LabelValue, copied.Labels[constants.LabelKey])
		assert.Equal(t, constants.RegistrySecretLabelValue, copied.Labels[constants.SecretLabelKey],
			"the copy must carry the registry-secret label so ImagePullSecretReconciler manages it")
	})

	t.Run("is a no-op when the copy already exists", func(t *testing.T) {
		existing := &corev1.Secret{
			Name:      testRegistrySecret,
			Namespace: testTargetNS,
			Labels: map[string]string{
				constants.LabelKey:       constants.LabelValue,
				constants.SecretLabelKey: constants.RegistrySecretLabelValue,
			},
			Data: map[string][]byte{DOCKERCONFIGJSON: []byte(`{"auths":{"already":{}}}`)},
		}
		c := newOwnedResourceClient(t, sourceRegistrySecret(), existing)

		require.NoError(t, ensureRegistrySecret(context.Background(), c,
			testRegistrySecret, testControllerNS, testTargetNS))

		copied := &corev1.Secret{}
		require.NoError(t, c.Get(context.Background(),
			types.NamespacedName{Namespace: testTargetNS, Name: testRegistrySecret}, copied))
		assert.Equal(t, []byte(`{"auths":{"already":{}}}`), copied.Data[DOCKERCONFIGJSON],
			"an existing copy must not be overwritten by ensure")
	})

	t.Run("is a no-op for the controller's own namespace", func(t *testing.T) {
		c := newOwnedResourceClient(t, sourceRegistrySecret())

		require.NoError(t, ensureRegistrySecret(context.Background(), c,
			testRegistrySecret, testControllerNS, testControllerNS))

		// Only the source remains; no second copy was created in the controller namespace.
		list := &corev1.SecretList{}
		require.NoError(t, c.List(context.Background(), list, client.InNamespace(testControllerNS)))
		assert.Len(t, list.Items, 1, "no copy should be created in the controller namespace")
	})

	t.Run("is a no-op when no registry secret is configured", func(t *testing.T) {
		c := newOwnedResourceClient(t)

		require.NoError(t, ensureRegistrySecret(context.Background(), c,
			"", testControllerNS, testTargetNS))

		assert.False(t, registrySecretExists(t, c, testTargetNS),
			"nothing should be created when the registry secret name is empty")
	})
}

func TestCleanupRegistrySecretIfLastTarget(t *testing.T) {
	cfg := matchingTargetConfig()

	copyInTargetNS := func() *corev1.Secret {
		return &corev1.Secret{
			Name:      testRegistrySecret,
			Namespace: testTargetNS,
			Labels: map[string]string{
				constants.LabelKey:       constants.LabelValue,
				constants.SecretLabelKey: constants.RegistrySecretLabelValue,
			},
			Data: map[string][]byte{DOCKERCONFIGJSON: []byte(`{"auths":{}}`)},
		}
	}

	t.Run("deletes the copy when the last target leaves the namespace", func(t *testing.T) {
		removed := targetDeployment("nginx", testTargetNS)
		// removed is the only target and is already gone from the API server.
		c := newOwnedResourceClient(t, copyInTargetNS())

		require.NoError(t, cleanupRegistrySecretIfLastTarget(context.Background(), c, cfg,
			testRegistrySecret, testControllerNS, removed))

		assert.False(t, registrySecretExists(t, c, testTargetNS),
			"the copy must be removed once no target remains in the namespace")
	})

	t.Run("keeps the copy while another target remains", func(t *testing.T) {
		removed := targetDeployment("nginx", testTargetNS)
		remaining := targetStatefulSet("nginx-sts", testTargetNS)
		c := newOwnedResourceClient(t, copyInTargetNS(), remaining)

		require.NoError(t, cleanupRegistrySecretIfLastTarget(context.Background(), c, cfg,
			testRegistrySecret, testControllerNS, removed))

		assert.True(t, registrySecretExists(t, c, testTargetNS),
			"the copy must be retained while another matched target lives in the namespace")
	})

	t.Run("excludes the terminating target even if still present in the cache", func(t *testing.T) {
		// The removed target is still returned by List (cache staleness); it must not
		// count toward "remaining targets" or the copy would never be cleaned up.
		removed := targetDeployment("nginx", testTargetNS)
		c := newOwnedResourceClient(t, copyInTargetNS(), removed)

		require.NoError(t, cleanupRegistrySecretIfLastTarget(context.Background(), c, cfg,
			testRegistrySecret, testControllerNS, removed))

		assert.False(t, registrySecretExists(t, c, testTargetNS),
			"a still-cached terminating target must be excluded from the remaining-target count")
	})

	t.Run("does not count targets in other namespaces", func(t *testing.T) {
		removed := targetDeployment("nginx", testTargetNS)
		otherNS := targetDeployment("nginx", "other-ns")
		c := newOwnedResourceClient(t, copyInTargetNS(), otherNS)

		require.NoError(t, cleanupRegistrySecretIfLastTarget(context.Background(), c, cfg,
			testRegistrySecret, testControllerNS, removed))

		assert.False(t, registrySecretExists(t, c, testTargetNS),
			"a target in a different namespace must not keep this namespace's copy alive")
	})

	t.Run("is a no-op when no registry secret is configured", func(t *testing.T) {
		removed := targetDeployment("nginx", testTargetNS)
		c := newOwnedResourceClient(t, copyInTargetNS())

		require.NoError(t, cleanupRegistrySecretIfLastTarget(context.Background(), c, cfg,
			"", testControllerNS, removed))

		assert.True(t, registrySecretExists(t, c, testTargetNS),
			"cleanup must not touch anything when the registry secret name is empty")
	})
}
