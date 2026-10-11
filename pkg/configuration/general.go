// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"strings"

	"github.com/nickytd/oidc-apps/pkg/randutils"
)

// GetHost return the domain name for a given workload target
func (rc *ResolvedConfig) GetHost() string {
	domain := rc.global.DomainName

	prefix := rc.object.GetName() + "-" + rc.object.GetNamespace()
	if rc.target.Ingress != nil && rc.target.Ingress.HostPrefix != "" {
		prefix = rc.target.Ingress.HostPrefix + "-" + randutils.GenerateSha256(rc.object.GetName()+"-"+rc.object.GetNamespace())
	}

	if rc.target.Ingress != nil && rc.target.Ingress.Host != "" {
		prefix, domain, _ = strings.Cut(rc.target.Ingress.Host, ".")
	}

	if domain == "" {
		return prefix
	}

	return strings.Join([]string{prefix, domain}, ".")
}

// GetUpstreamTarget returns the protocol and port tuple of the target workload
func (rc *ResolvedConfig) GetUpstreamTarget() string {
	b := strings.Builder{}
	protocol := "http"

	if rc.target.TargetProtocol == "https" {
		protocol = "https"
	}

	b.Grow(9)
	_, _ = b.WriteString("protocol=")
	b.Grow(len(protocol))
	_, _ = b.WriteString(protocol)
	b.Grow(7)
	_, _ = b.WriteString(", port=")
	b.Grow(len(rc.target.TargetPort.String()))
	_, _ = b.WriteString(rc.target.TargetPort.String())

	return b.String()
}
