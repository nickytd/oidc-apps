// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/nickytd/oidc-apps/pkg/constants"
)

const (
	testRegistrySecret = "registry-secret"
	testControllerNS   = "oidc-apps-system"
	testTargetNS       = "workload-ns"
)

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()

	s := scheme.Scheme
	require.NoError(t, corev1.AddToScheme(s))

	return fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objs...).
		Build()
}

func sourceSecret(data string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testRegistrySecret,
			Namespace: testControllerNS,
		},
		Data: map[string][]byte{
			DOCKERCONFIGJSON: []byte(data),
		},
	}
}

func labeledTarget(name, _ string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testTargetNS,
			Labels: map[string]string{
				constants.LabelKey:       constants.LabelValue,
				constants.SecretLabelKey: constants.RegistrySecretLabelValue,
			},
		},
		Data: map[string][]byte{
			DOCKERCONFIGJSON: []byte("stale"),
		},
	}
}

func TestImagePullSecretReconcile(t *testing.T) {
	t.Run("propagates rotated data to labeled targets", func(t *testing.T) {
		src := sourceSecret("new-credentials")
		target := labeledTarget("target-secret", "stale")
		c := newFakeClient(t, src, target)

		r := &ImagePullSecretReconciler{Client: c, SecretName: testRegistrySecret}

		_, err := r.Reconcile(context.Background(), reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: testControllerNS, Name: testRegistrySecret},
		})
		require.NoError(t, err)

		got := &corev1.Secret{}
		require.NoError(t, c.Get(context.Background(),
			types.NamespacedName{Namespace: testTargetNS, Name: "target-secret"}, got))
		assert.Equal(t, []byte("new-credentials"), got.Data[DOCKERCONFIGJSON],
			"target should receive the rotated credentials")

		gotSrc := &corev1.Secret{}
		require.NoError(t, c.Get(context.Background(),
			types.NamespacedName{Namespace: testControllerNS, Name: testRegistrySecret}, gotSrc))
		assert.Equal(t, []byte("new-credentials"), gotSrc.Data[DOCKERCONFIGJSON],
			"source secret must be left unchanged")
	})

	t.Run("ignores reconcile requests for a different secret name", func(t *testing.T) {
		src := sourceSecret("new-credentials")
		target := labeledTarget("target-secret", "stale")
		c := newFakeClient(t, src, target)

		r := &ImagePullSecretReconciler{Client: c, SecretName: testRegistrySecret}

		_, err := r.Reconcile(context.Background(), reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: testControllerNS, Name: "some-other-secret"},
		})
		require.NoError(t, err)

		got := &corev1.Secret{}
		require.NoError(t, c.Get(context.Background(),
			types.NamespacedName{Namespace: testTargetNS, Name: "target-secret"}, got))
		assert.Equal(t, []byte("stale"), got.Data[DOCKERCONFIGJSON],
			"targets must be untouched when the request name does not match")
	})

	t.Run("no labeled targets is a no-op without error", func(t *testing.T) {
		src := sourceSecret("new-credentials")
		c := newFakeClient(t, src)

		r := &ImagePullSecretReconciler{Client: c, SecretName: testRegistrySecret}

		_, err := r.Reconcile(context.Background(), reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: testControllerNS, Name: testRegistrySecret},
		})
		require.NoError(t, err)
	})
}
