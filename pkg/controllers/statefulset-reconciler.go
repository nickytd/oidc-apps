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
	Config *configuration.OIDCAppsControllerConfig
	// RegistrySecretName is the configured private registry secret to propagate into target
	// namespaces; empty when no private registry is configured (propagation disabled).
	RegistrySecretName string
	// ControllerNamespace is the namespace holding the source registry secret.
	ControllerNamespace string
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

	if !s.Config.Match(ctx, reconciledStatefulSet) {
		_log.V(9).Info("Reconciled statefulset is no longer an oidc-application-controller target, pruning owned resources ...")

		// The statefulset still exists but no longer matches the targeting
		// configuration, so owner-reference garbage collection will not fire.
		// Prune the previously created owned resources explicitly.
		if err := deleteOwnedResources(ctx, s.Client, s.Config, reconciledStatefulSet); err != nil {
			return reconcile.Result{}, err
		}

		// Remove the propagated registry secret if this was the last target in the namespace.
		err := cleanupRegistrySecretIfLastTarget(ctx, s.Client, s.Config,
			s.RegistrySecretName, s.ControllerNamespace, reconciledStatefulSet)
		if err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// A matched target being deleted will not re-enter the dependency path (it is a no-op once the
	// deletion timestamp is set), so clean up the registry secret here if it was the last target.
	if !reconciledStatefulSet.GetDeletionTimestamp().IsZero() {
		err := cleanupRegistrySecretIfLastTarget(ctx, s.Client, s.Config,
			s.RegistrySecretName, s.ControllerNamespace, reconciledStatefulSet)
		if err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	if !hasOidcAppsPods(ctx, s.Client, reconciledStatefulSet) {
		return reconcile.Result{}, nil
	}

	// Propagate the private registry secret into the target namespace so the injected sidecar
	// images can be pulled. No-op when no registry secret is configured.
	err := ensureRegistrySecret(ctx, s.Client,
		s.RegistrySecretName, s.ControllerNamespace, reconciledStatefulSet.GetNamespace())
	if err != nil {
		return reconcile.Result{}, err
	}

	// On deletion the owned resources carry owner references to this statefulset
	// (and its pods), so Kubernetes garbage collection removes them automatically.
	// The dependency reconcile below is a no-op once the deletion timestamp is set.
	if err := reconcileStatefulSetDependencies(ctx, s.Client, s.Config, reconciledStatefulSet); err != nil {
		return reconcile.Result{}, err
	}

	_log.Info("reconciled statefulset successfully")

	return reconcile.Result{}, nil
}
