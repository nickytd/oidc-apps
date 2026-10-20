// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestDeploymentReconcile(t *testing.T) {
	// With no controller configuration loaded, Match returns false for every object,
	// so the reconciler takes the de-targeting prune branch. This exercises the case
	// where a deployment is de-targeted by stripping its labels (GetLabels() empty):
	// the prune must still fire and must not be skipped by a non-nil-labels guard.
	t.Run("prunes owned resources when a de-targeted deployment has no labels", func(t *testing.T) {
		owner := ownerDeployment("owner-uid")
		owner.Labels = nil
		owned := ownedSecret("owned", "owner-uid")
		c := newOwnedResourceClient(t, owner, owned)

		d := &DeploymentReconciler{Client: c}

		_, err := d.Reconcile(context.Background(), reconcile.Request{
			Namespace: testTargetNS, Name: "owner",
		})
		require.NoError(t, err)

		err = c.Get(context.Background(),
			types.NamespacedName{Namespace: testTargetNS, Name: "owned"}, &corev1.Secret{})
		assert.True(t, apierrors.IsNotFound(err),
			"owned secret of a de-targeted deployment must be pruned even when the deployment has no labels")
	})

	t.Run("prunes owned resources when a de-targeted deployment still has labels", func(t *testing.T) {
		owner := ownerDeployment("owner-uid")
		owner.Labels = map[string]string{"app": "no-longer-matching"}
		owned := ownedSecret("owned", "owner-uid")
		c := newOwnedResourceClient(t, owner, owned)

		d := &DeploymentReconciler{Client: c}

		_, err := d.Reconcile(context.Background(), reconcile.Request{
			Namespace: testTargetNS, Name: "owner",
		})
		require.NoError(t, err)

		err = c.Get(context.Background(),
			types.NamespacedName{Namespace: testTargetNS, Name: "owned"}, &corev1.Secret{})
		assert.True(t, apierrors.IsNotFound(err),
			"owned secret of a de-targeted deployment must be pruned")
	})

	t.Run("is a no-op for an empty reconcile request", func(t *testing.T) {
		c := newOwnedResourceClient(t)

		d := &DeploymentReconciler{Client: c}

		_, err := d.Reconcile(context.Background(), reconcile.Request{
			Namespace: testTargetNS, Name: "missing",
		})
		require.NoError(t, err, "reconciling a non-existent deployment must not error")
	})
}
