// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/nickytd/oidc-apps/pkg/configuration"
)

// StatefulSetReconciler holds configuration for the reconciler
type StatefulSetReconciler struct {
	Client client.Client
}

// Reconcile creates the auth & zutz secrets mounted to the target statefulset
func (s *StatefulSetReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reconciledStatefulSet := &appsv1.StatefulSet{}

	if err := s.Client.Get(ctx, request.NamespacedName, reconciledStatefulSet); client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, err
	}

	_log := log.FromContext(ctx).WithValues("resourceVersion", reconciledStatefulSet.GetResourceVersion())

	if reconciledStatefulSet.GetName() == "" && reconciledStatefulSet.GetNamespace() == "" {
		_log.V(9).Info("Reconciled statefulset is empty, returning ...")

		return reconcile.Result{}, nil
	}

	_log.V(9).Info("handling statefulset reconcile request")

	if !configuration.GetOIDCAppsControllerConfig().Match(reconciledStatefulSet) {
		_log.V(9).Info("Reconciled statefulset is no longer an oidc-application-controller target, pruning owned resources ...")

		// The statefulset still exists but no longer matches the targeting
		// configuration, so owner-reference garbage collection will not fire.
		// Prune the previously created owned resources explicitly.
		if err := deleteOwnedResources(ctx, s.Client, reconciledStatefulSet); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	if !hasOidcAppsPods(ctx, s.Client, reconciledStatefulSet) {
		return reconcile.Result{}, nil
	}

	// On deletion the owned resources carry owner references to this statefulset
	// (and its pods), so Kubernetes garbage collection removes them automatically.
	// The dependency reconcile below is a no-op once the deletion timestamp is set.
	if err := reconcileStatefulSetDependencies(ctx, s.Client, reconciledStatefulSet); err != nil {
		return reconcile.Result{}, err
	}

	_log.Info("reconciled statefulset successfully")

	return reconcile.Result{}, nil
}
