// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import "sigs.k8s.io/controller-runtime/pkg/client"

// GetKubeSecretName returns the kubeconfig secret name of the target workload
func (c *OIDCAppsControllerConfig) GetKubeSecretName(object client.Object) string {
	secretName := ""

	t := c.FetchTarget(object)
	if t.KubeRbacProxy != nil &&
		t.KubeRbacProxy.KubeSecretRef != nil &&
		t.KubeRbacProxy.KubeSecretRef.Name != "" {
		return t.KubeRbacProxy.KubeSecretRef.Name
	}

	if c.Global.KubeRbacProxy != nil &&
		c.Global.KubeRbacProxy.KubeSecretRef != nil &&
		c.Global.KubeRbacProxy.KubeSecretRef.Name != "" {
		secretName = c.Global.KubeRbacProxy.KubeSecretRef.Name
	}

	return secretName
}

// GetKubeConfigStr returns the kubeconfig string of the target workload
func (c *OIDCAppsControllerConfig) GetKubeConfigStr(object client.Object) string {
	kubeConfig := ""

	t := c.FetchTarget(object)
	if t.KubeRbacProxy != nil &&
		t.KubeRbacProxy.KubeConfigStr != "" {
		return t.KubeRbacProxy.KubeConfigStr
	}

	if c.Global.KubeRbacProxy != nil &&
		c.Global.KubeRbacProxy.KubeConfigStr != "" {
		kubeConfig = c.Global.KubeRbacProxy.KubeConfigStr
	}

	return kubeConfig
}
