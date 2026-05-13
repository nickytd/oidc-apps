// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nickytd/oidc-apps/pkg/randutils"
)

// GetHost return the domain name for a given workload target
func (c *OIDCAppsControllerConfig) GetHost(object client.Object) string {
	t := c.FetchTarget(object)
	domain := c.Global.DomainName

	prefix := object.GetName() + "-" + object.GetNamespace()
	if t.Ingress != nil && t.Ingress.HostPrefix != "" {
		prefix = t.Ingress.HostPrefix + "-" + randutils.GenerateSha256(object.GetName()+"-"+object.GetNamespace())
	}

	if t.Ingress != nil && t.Ingress.Host != "" {
		prefix, domain, _ = strings.Cut(t.Ingress.Host, ".")
	}

	if domain == "" {
		return prefix
	}

	return strings.Join([]string{prefix, domain}, ".")
}

// GetUpstreamTarget returns the protocol and port tuple of the target workload
func (c *OIDCAppsControllerConfig) GetUpstreamTarget(object client.Object) string {
	b := strings.Builder{}
	protocol := "http"

	t := c.FetchTarget(object)
	if t.TargetProtocol == "https" {
		protocol = "https"
	}

	b.Grow(9)
	_, _ = b.WriteString("protocol=")
	b.Grow(len(protocol))
	_, _ = b.WriteString(protocol)
	b.Grow(7)
	_, _ = b.WriteString(", port=")
	b.Grow(len(t.TargetPort.String()))
	_, _ = b.WriteString(t.TargetPort.String())

	return b.String()
}
