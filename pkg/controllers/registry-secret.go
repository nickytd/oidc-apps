// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
)

// ensureRegistrySecret creates a labeled copy of the configured registry secret in the target's
// namespace when a matched target lives there. It is a no-op when no registry secret is configured,
// when the target namespace is the controller's own namespace (the source already lives there), or
// when the copy already exists. The copy carries the registry-secret label so the
// ImagePullSecretReconciler manages its rotation and source-deletion cleanup.
func ensureRegistrySecret(ctx context.Context, c client.Client,
	registrySecretName, controllerNamespace, targetNamespace string) error {
	if registrySecretName == "" || targetNamespace == controllerNamespace {
		return nil
	}

	// Skip if the copy already exists in the target namespace.
	existing := &corev1.Secret{}

	err := c.Get(ctx, client.ObjectKey{Namespace: targetNamespace, Name: registrySecretName}, existing)
	if err == nil {
		return nil
	}

	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to check for existing registry secret in %s: %w", targetNamespace, err)
	}

	source := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: controllerNamespace, Name: registrySecretName}, source); err != nil {
		return fmt.Errorf("failed to get source registry secret %s/%s: %w",
			controllerNamespace, registrySecretName, client.IgnoreNotFound(err))
	}

	copied := &corev1.Secret{
		ObjectMeta: metaForRegistrySecretCopy(registrySecretName, targetNamespace),
		Data:       source.Data,
		Type:       source.Type,
	}

	if err := c.Create(ctx, copied); client.IgnoreAlreadyExists(err) != nil {
		return fmt.Errorf("failed to create registry secret copy in %s: %w", targetNamespace, err)
	}

	log.FromContext(ctx).Info("propagated registry secret", "namespace", targetNamespace)

	return nil
}

// cleanupRegistrySecretIfLastTarget removes the registry secret copy from removedTarget's namespace
// once no matched target remains there. The removedTarget is excluded from the remaining-target
// count by name+namespace, so a still-cached terminating object does not keep the copy alive. It is
// a no-op when no registry secret is configured or for the controller's own namespace.
func cleanupRegistrySecretIfLastTarget(ctx context.Context, c client.Client,
	cfg *configuration.OIDCAppsControllerConfig,
	registrySecretName, controllerNamespace string, removedTarget client.Object) error {
	namespace := removedTarget.GetNamespace()
	if registrySecretName == "" || namespace == controllerNamespace {
		return nil
	}

	remaining, err := hasMatchedTargetInNamespace(ctx, c, cfg, namespace, removedTarget)
	if err != nil {
		return err
	}

	if remaining {
		return nil
	}

	copied := &corev1.Secret{ObjectMeta: metaForRegistrySecretCopy(registrySecretName, namespace)}
	if err := c.Delete(ctx, copied); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete registry secret copy in %s: %w", namespace, err)
	}

	log.FromContext(ctx).Info("removed propagated registry secret", "namespace", namespace)

	return nil
}

// hasMatchedTargetInNamespace reports whether any Deployment or StatefulSet in the namespace still
// matches the targeting configuration, excluding the object currently being removed.
func hasMatchedTargetInNamespace(ctx context.Context, c client.Client,
	cfg *configuration.OIDCAppsControllerConfig, namespace string, removed client.Object) (bool, error) {
	deployments := &appsv1.DeploymentList{}
	if err := c.List(ctx, deployments, client.InNamespace(namespace)); err != nil {
		return false, fmt.Errorf("failed to list deployments in %s: %w", namespace, err)
	}

	for i := range deployments.Items {
		d := &deployments.Items[i]
		if isSameObject(d, removed) {
			continue
		}

		if cfg.Match(ctx, d) {
			return true, nil
		}
	}

	statefulsets := &appsv1.StatefulSetList{}
	if err := c.List(ctx, statefulsets, client.InNamespace(namespace)); err != nil {
		return false, fmt.Errorf("failed to list statefulsets in %s: %w", namespace, err)
	}

	for i := range statefulsets.Items {
		s := &statefulsets.Items[i]
		if isSameObject(s, removed) {
			continue
		}

		if cfg.Match(ctx, s) {
			return true, nil
		}
	}

	return false, nil
}

// isSameObject reports whether a and b denote the same workload. It compares the concrete Go type
// alongside namespace and name so it stays correct even when the fake client (or a cache read)
// leaves TypeMeta/Kind unset, and so a Deployment never aliases a StatefulSet of the same name.
func isSameObject(a, b client.Object) bool {
	return fmt.Sprintf("%T", a) == fmt.Sprintf("%T", b) &&
		a.GetNamespace() == b.GetNamespace() &&
		a.GetName() == b.GetName()
}

func metaForRegistrySecretCopy(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels: map[string]string{
			constants.LabelKey:       constants.LabelValue,
			constants.SecretLabelKey: constants.RegistrySecretLabelValue,
		},
	}
}
