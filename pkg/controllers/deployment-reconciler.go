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

// DeploymentReconciler holds configuration for the reconciler
type DeploymentReconciler struct {
	Client client.Client
	Config *configuration.OIDCAppsControllerConfig
	// RegistrySecretName is the configured private registry secret to propagate into target
	// namespaces; empty when no private registry is configured (propagation disabled).
	RegistrySecretName string
	// ControllerNamespace is the namespace holding the source registry secret.
	ControllerNamespace string
}

// Reconcile creates the auth & zutz secrets mounted to the target deployment
func (d *DeploymentReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reconciledDeployment := &appsv1.Deployment{}
	if err := d.Client.Get(ctx, request.NamespacedName, reconciledDeployment); client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, err
	}

	_log := log.FromContext(ctx).WithValues("resourceVersion", reconciledDeployment.GetResourceVersion())

	// Skip resource without an identity
	if reconciledDeployment.GetName() == "" && reconciledDeployment.GetNamespace() == "" {
		_log.V(9).Info("reconciled deployment is empty, returning ...")

		return reconcile.Result{}, nil
	}

	_log.V(9).Info("handling deployment reconcile request")

	if !d.Config.Match(ctx, reconciledDeployment) {
		_log.V(9).Info("reconciled deployment is no longer an oidc-application-controller target, pruning owned resources ...")

		// The deployment still exists but no longer matches the targeting
		// configuration, so owner-reference garbage collection will not fire.
		// Prune the previously created owned resources explicitly. This also covers
		// de-targeting by removing the matching labels, which leaves GetLabels() empty.
		if err := deleteOwnedResources(ctx, d.Client, d.Config, reconciledDeployment); err != nil {
			return reconcile.Result{}, err
		}

		// Remove the propagated registry secret if this was the last target in the namespace.
		err := cleanupRegistrySecretIfLastTarget(ctx, d.Client, d.Config,
			d.RegistrySecretName, d.ControllerNamespace, reconciledDeployment)
		if err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// A matched target being deleted will not re-enter the dependency path (it is a no-op once the
	// deletion timestamp is set), so clean up the registry secret here if it was the last target.
	if !reconciledDeployment.GetDeletionTimestamp().IsZero() {
		err := cleanupRegistrySecretIfLastTarget(ctx, d.Client, d.Config,
			d.RegistrySecretName, d.ControllerNamespace, reconciledDeployment)
		if err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	if !hasOidcAppsPods(ctx, d.Client, reconciledDeployment) {
		return reconcile.Result{}, nil
	}

	// Propagate the private registry secret into the target namespace so the injected sidecar
	// images can be pulled. No-op when no registry secret is configured.
	err := ensureRegistrySecret(ctx, d.Client,
		d.RegistrySecretName, d.ControllerNamespace, reconciledDeployment.GetNamespace())
	if err != nil {
		return reconcile.Result{}, err
	}

	// On deletion the owned resources carry owner references to this deployment,
	// so Kubernetes garbage collection removes them automatically. The dependency
	// reconcile below is a no-op once the deletion timestamp is set.
	if err := reconcileDeploymentDependencies(ctx, d.Client, d.Config, reconciledDeployment); err != nil {
		return reconcile.Result{}, err
	}

	_log.Info("reconciled deployment successfully")

	return reconcile.Result{}, nil
}
