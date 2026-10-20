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
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
)

func ownerDeployment(uid string) *appsv1.Deployment {
	return &appsv1.Deployment{
		Name:      "owner",
		Namespace: testTargetNS,
		UID:       types.UID(uid),
	}
}

func ownedSecret(name, ownerUID string) *corev1.Secret {
	s := &corev1.Secret{
		Name:      name,
		Namespace: testTargetNS,
		Labels: map[string]string{
			constants.LabelKey:       constants.LabelValue,
			constants.SecretLabelKey: constants.Oauth2LabelValue,
		},
	}
	if ownerUID != "" {
		s.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "owner",
			UID:        types.UID(ownerUID),
		}}
	}

	return s
}

func ownedService(name, ownerUID string) *corev1.Service {
	return &corev1.Service{
		Name:      name,
		Namespace: testTargetNS,
		Labels:    map[string]string{constants.LabelKey: constants.LabelValue},
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "owner",
			UID:        types.UID(ownerUID),
		}},
	}
}

func newOwnedResourceClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()

	s := scheme.Scheme
	require.NoError(t, corev1.AddToScheme(s))
	require.NoError(t, appsv1.AddToScheme(s))
	require.NoError(t, gatewayv1.Install(s))

	return fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objs...).
		Build()
}

func TestDeleteOwnedResources(t *testing.T) {
	t.Run("deletes only resources owned by the given object", func(t *testing.T) {
		owner := ownerDeployment("owner-uid")
		mine := ownedSecret("mine", "owner-uid")
		theirs := ownedSecret("theirs", "another-uid")
		myService := ownedService("my-svc", "owner-uid")
		c := newOwnedResourceClient(t, owner, mine, theirs, myService)

		require.NoError(t, deleteOwnedResources(context.Background(), c, &configuration.OIDCAppsControllerConfig{}, owner))

		// The resource owned by this deployment must be gone.
		err := c.Get(context.Background(),
			types.NamespacedName{Namespace: testTargetNS, Name: "mine"}, &corev1.Secret{})
		assert.True(t, apierrors.IsNotFound(err), "owned secret should be deleted")

		err = c.Get(context.Background(),
			types.NamespacedName{Namespace: testTargetNS, Name: "my-svc"}, &corev1.Service{})
		assert.True(t, apierrors.IsNotFound(err), "owned service should be deleted")

		// The resource owned by a different object must be left untouched.
		require.NoError(t, c.Get(context.Background(),
			types.NamespacedName{Namespace: testTargetNS, Name: "theirs"}, &corev1.Secret{}),
			"secret owned by a different workload must not be deleted")
	})

	t.Run("is idempotent when there is nothing to delete", func(t *testing.T) {
		owner := ownerDeployment("owner-uid")
		c := newOwnedResourceClient(t, owner)

		require.NoError(t, deleteOwnedResources(context.Background(), c, &configuration.OIDCAppsControllerConfig{}, owner),
			"pruning with no owned resources must not error")
	})
}
