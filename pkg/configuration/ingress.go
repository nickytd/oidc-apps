// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"regexp"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ShallCreateIngress returns true if the target workload shall create an ingress
func (c *OIDCAppsControllerConfig) ShallCreateIngress(object client.Object) bool {
	t := c.FetchTarget(object)
	if t.Ingress != nil {
		return t.Ingress.Create
	}

	return false
}

// GetIngressTLSSecretName return the tls secret for the ingress serving certificate for the given workload
func (c *OIDCAppsControllerConfig) GetIngressTLSSecretName(object client.Object) string {
	t := c.FetchTarget(object)
	if t.Ingress != nil && t.Ingress.TLSSecretRef.Name != "" {
		return t.Ingress.TLSSecretRef.Name
	}

	return ""
}

// GetIngressClassName return the ingress class name for the given target
func (c *OIDCAppsControllerConfig) GetIngressClassName(object client.Object) string {
	t := c.FetchTarget(object)
	if t.Ingress != nil {
		return t.Ingress.IngressClassName
	}

	return ""
}

// GetIngressAnnotations returns the ingress annotations for the given target
func (c *OIDCAppsControllerConfig) GetIngressAnnotations(object client.Object) map[string]string {
	t := c.FetchTarget(object)
	if t.Ingress != nil && t.Ingress.Annotations != nil {
		return t.Ingress.Annotations
	}

	return nil
}

// GetIngressLabels returns the ingress labels for the given target
func (c *OIDCAppsControllerConfig) GetIngressLabels(object client.Object) map[string]string {
	t := c.FetchTarget(object)
	if t.Ingress != nil && t.Ingress.Labels != nil {
		return t.Ingress.Labels
	}

	return nil
}

// GetIngressDefaultPath returns the default redirect path for the ingress of the given target.
// The path must start with "/" to be valid; invalid paths are ignored.
func (c *OIDCAppsControllerConfig) GetIngressDefaultPath(object client.Object) string {
	t := c.FetchTarget(object)
	if t.Ingress != nil && isValidDefaultPath(t.Ingress.DefaultPath) {
		return t.Ingress.DefaultPath
	}

	return ""
}

const maxDefaultPathLength = 32

var validDefaultPathRe = regexp.MustCompile(`^/[-a-zA-Z0-9/_.~]+$`)

func isValidDefaultPath(path string) bool {
	return len(path) <= maxDefaultPathLength && validDefaultPathRe.MatchString(path)
}
