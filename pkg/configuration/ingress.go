// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

// ShallCreateIngress returns true if the target workload shall create an ingress
func (rc *ResolvedConfig) ShallCreateIngress() bool {
	if rc.target.Ingress != nil {
		return rc.target.Ingress.Create
	}

	return false
}

// GetIngressTLSSecretName return the tls secret for the ingress serving certificate for the given workload
func (rc *ResolvedConfig) GetIngressTLSSecretName() string {
	if rc.target.Ingress != nil && rc.target.Ingress.TLSSecretRef.Name != "" {
		return rc.target.Ingress.TLSSecretRef.Name
	}

	return ""
}

// GetIngressClassName return the ingress class name for the given target
func (rc *ResolvedConfig) GetIngressClassName() string {
	if rc.target.Ingress != nil {
		return rc.target.Ingress.IngressClassName
	}

	return ""
}

// GetIngressAnnotations returns the ingress annotations for the given target
func (rc *ResolvedConfig) GetIngressAnnotations() map[string]string {
	if rc.target.Ingress != nil && rc.target.Ingress.Annotations != nil {
		return rc.target.Ingress.Annotations
	}

	return nil
}

// GetIngressLabels returns the ingress labels for the given target
func (rc *ResolvedConfig) GetIngressLabels() map[string]string {
	if rc.target.Ingress != nil && rc.target.Ingress.Labels != nil {
		return rc.target.Ingress.Labels
	}

	return nil
}

// GetIngressDefaultPath returns the default redirect path for the ingress of the given target.
// The path must start with "/" to be valid; invalid paths are ignored.
func (rc *ResolvedConfig) GetIngressDefaultPath() string {
	if rc.target.Ingress != nil && isValidDefaultPath(rc.target.Ingress.DefaultPath) {
		return rc.target.Ingress.DefaultPath
	}

	return ""
}
