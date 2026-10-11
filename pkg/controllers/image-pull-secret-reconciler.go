// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/nickytd/oidc-apps/pkg/constants"
)

const (
	// DOCKERCONFIGJSON is a standard field name in the authentication secrets for private container registries
	DOCKERCONFIGJSON = ".dockerconfigjson"
)

// ImagePullSecretReconciler holds configuration for the reconciler
type ImagePullSecretReconciler struct {
	Client     client.Client
	SecretName string
}

// Reconcile propagates the private registry secrets through the namespaces
func (r *ImagePullSecretReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	_log := log.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, request.NamespacedName, secret); client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, err
	}

	if secret.GetName() != r.SecretName {
		return reconcile.Result{}, nil
	}

	// The propagated copies live in other namespaces and cannot carry owner references
	// back to this source secret, so a finalizer drives their cleanup on deletion.
	if !secret.GetDeletionTimestamp().IsZero() {
		if !controllerutil.ContainsFinalizer(secret, constants.RegistrySecretFinalizer) {
			return reconcile.Result{}, nil
		}

		if err := r.deletePropagatedSecrets(ctx, secret.GetNamespace()); err != nil {
			return reconcile.Result{}, err
		}

		controllerutil.RemoveFinalizer(secret, constants.RegistrySecretFinalizer)

		if err := r.Client.Update(ctx, secret); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}

		return reconcile.Result{}, nil
	}

	if controllerutil.AddFinalizer(secret, constants.RegistrySecretFinalizer) {
		if err := r.Client.Update(ctx, secret); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	secretsList := &corev1.SecretList{}

	err := r.Client.List(ctx, secretsList,
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(
				map[string]string{
					constants.LabelKey:       constants.LabelValue,
					constants.SecretLabelKey: constants.RegistrySecretLabelValue,
				},
			),
		},
	)
	if err != nil {
		_log.Error(err, "Error fetching image pull secrets")

		return reconcile.Result{}, err
	}

	for i := range secretsList.Items {
		target := &secretsList.Items[i]

		if target.Data == nil {
			target.Data = map[string][]byte{}
		}

		target.Data[DOCKERCONFIGJSON] = secret.Data[DOCKERCONFIGJSON]

		if err := r.Client.Update(ctx, target); err != nil {
			_log.Error(err, "Cannot update image pull secret",
				"name", target.GetName(),
				"namespace", target.GetNamespace(),
			)

			continue
		}

		_log.V(9).Info("Updated", "name", target.GetName(), "namespace", target.GetNamespace())
	}

	return reconcile.Result{}, nil
}

// deletePropagatedSecrets removes the propagated registry secret copies in every namespace
// except sourceNamespace (which holds the source secret being deleted).
func (r *ImagePullSecretReconciler) deletePropagatedSecrets(ctx context.Context, sourceNamespace string) error {
	_log := log.FromContext(ctx)

	secretsList := &corev1.SecretList{}

	err := r.Client.List(ctx, secretsList,
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(
				map[string]string{
					constants.LabelKey:       constants.LabelValue,
					constants.SecretLabelKey: constants.RegistrySecretLabelValue,
				},
			),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to list propagated image pull secrets: %w", err)
	}

	for i := range secretsList.Items {
		target := &secretsList.Items[i]

		// Never delete the source secret itself; only the propagated copies.
		if target.GetNamespace() == sourceNamespace {
			continue
		}

		if err := r.Client.Delete(ctx, target); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete propagated image pull secret %s/%s: %w",
				target.GetNamespace(), target.GetName(), err)
		}

		_log.V(9).Info("Deleted propagated image pull secret",
			"name", target.GetName(), "namespace", target.GetNamespace())
	}

	return nil
}
