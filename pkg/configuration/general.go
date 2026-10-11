// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"regexp"
	"strings"

	"github.com/nickytd/oidc-apps/pkg/randutils"
)

// GetHost return the domain name for a given workload target
func (rc *ResolvedConfig) GetHost() string {
	domain := rc.global.DomainName

	prefix := rc.object.GetName() + "-" + rc.object.GetNamespace()
	if rc.target.HTTPRoute != nil && rc.target.HTTPRoute.HostPrefix != "" {
		prefix = rc.target.HTTPRoute.HostPrefix + "-" + randutils.GenerateSha256(rc.object.GetName()+"-"+rc.object.GetNamespace())
	}

	if rc.target.HTTPRoute != nil && rc.target.HTTPRoute.Host != "" {
		prefix, domain, _ = strings.Cut(rc.target.HTTPRoute.Host, ".")
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

const maxDefaultPathLength = 32

var validDefaultPathRe = regexp.MustCompile(`^/[-a-zA-Z0-9/_.~]+$`)

func isValidDefaultPath(path string) bool {
	return len(path) <= maxDefaultPathLength && validDefaultPathRe.MatchString(path)
}
