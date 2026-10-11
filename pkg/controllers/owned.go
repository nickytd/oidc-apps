// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/nickytd/oidc-apps/pkg/configuration"
)

func isAnOwnedResource(owner, owned client.Object) bool {
	if owner == nil || owned == nil {
		return false
	}

	for _, ref := range owned.GetOwnerReferences() {
		if ref.UID == owner.GetUID() {
			return true
		}
	}

	return false
}

func deleteOwnedResources(ctx context.Context, c client.Client, cfg *configuration.OIDCAppsControllerConfig, object client.Object) error {
	var err error

	_log := log.FromContext(ctx).WithValues("uid", object.GetUID())

	secrets, err := fetchOidcAppsSecrets(ctx, c, object)
	if err != nil {
		return err
	}

	for _, s := range secrets.Items {
		if err = c.Delete(ctx, &s); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete secret %s/%s: %w", s.Namespace, s.Name, err)
		}

		_log.V(9).Info("Deleted", "name", s.Name, "namespace", s.Namespace)
	}

	ingresses, err := fetchOidcAppsIngress(ctx, c, object)
	if err != nil {
		return err
	}

	for _, s := range ingresses.Items {
		if err = c.Delete(ctx, &s); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete ingress %s/%s: %w", s.Namespace, s.Name, err)
		}

		_log.V(9).Info("Deleted", "name", s.Name, "namespace", s.Namespace)
	}

	httpRoutes, err := fetchOidcAppsHTTPRoutes(ctx, c, cfg, object)
	if err != nil {
		return err
	}

	for _, s := range httpRoutes.Items {
		if err = c.Delete(ctx, &s); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete httproute %s/%s: %w", s.Namespace, s.Name, err)
		}

		_log.V(9).Info("Deleted", "name", s.Name, "namespace", s.Namespace)
	}

	services, err := fetchOidcAppsServices(ctx, c, object)
	if err != nil {
		return err
	}

	for _, s := range services.Items {
		if err = c.Delete(ctx, &s); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete service %s/%s: %w", s.Namespace, s.Name, err)
		}

		_log.V(9).Info("Deleted", "name", s.Name, "namespace", s.Namespace)
	}

	return nil
}
