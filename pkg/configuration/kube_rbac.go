// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

// GetKubeSecretName returns the kubeconfig secret name of the target workload
func (rc *ResolvedConfig) GetKubeSecretName() string {
	secretName := ""

	if rc.target.KubeRbacProxy != nil &&
		rc.target.KubeRbacProxy.KubeSecretRef != nil &&
		rc.target.KubeRbacProxy.KubeSecretRef.Name != "" {
		return rc.target.KubeRbacProxy.KubeSecretRef.Name
	}

	if rc.global.KubeRbacProxy != nil &&
		rc.global.KubeRbacProxy.KubeSecretRef != nil &&
		rc.global.KubeRbacProxy.KubeSecretRef.Name != "" {
		secretName = rc.global.KubeRbacProxy.KubeSecretRef.Name
	}

	return secretName
}

// GetKubeConfigStr returns the kubeconfig string of the target workload
func (rc *ResolvedConfig) GetKubeConfigStr() string {
	kubeConfig := ""

	if rc.target.KubeRbacProxy != nil &&
		rc.target.KubeRbacProxy.KubeConfigStr != "" {
		return rc.target.KubeRbacProxy.KubeConfigStr
	}

	if rc.global.KubeRbacProxy != nil &&
		rc.global.KubeRbacProxy.KubeConfigStr != "" {
		kubeConfig = rc.global.KubeRbacProxy.KubeConfigStr
	}

	return kubeConfig
}
