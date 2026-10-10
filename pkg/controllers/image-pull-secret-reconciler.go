// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
