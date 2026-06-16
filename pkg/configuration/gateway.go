// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nickytd/oidc-apps/pkg/constants"
	"github.com/nickytd/oidc-apps/pkg/randutils"
)

// GatewayGlobalConf holds global configuration for Gateway API support
type GatewayGlobalConf struct {
	// HTTPRoutes controls whether HTTPRoute resources are created
	HTTPRoutes *HTTPRoutesConf `json:"httpRoutes,omitzero"`
	// Managed controls whether the controller creates and manages a Gateway resource
	Managed bool `json:"managed,omitzero"`
	// Name is the name of the managed Gateway (defaults to constants.ManagedGatewayName)
	Name string `json:"name,omitzero"`
	// GatewayClassName is the GatewayClass to reference (required when managed is true)
	GatewayClassName string `json:"gatewayClassName,omitzero"`
	// Listeners associated with this Gateway
	Listeners []GatewayListener `json:"listeners,omitzero"`
	// Annotations for the managed Gateway
	Annotations map[string]string `json:"annotations,omitzero"`
	// Labels for the managed Gateway
	Labels map[string]string `json:"labels,omitzero"`
	// Infrastructure mirrors gateway.networking.k8s.io/v1.GatewayInfrastructure.
	// Unlike Annotations/Labels above (which apply to the Gateway resource itself),
	// these flow to the resources the gateway-controller generates (e.g. the
	// LoadBalancer Service / Pod). Use ParametersRef to override Service-level
	// fields such as ipFamilyPolicy via a JSON-merge-patch ConfigMap.
	Infrastructure *GatewayInfrastructure `json:"infrastructure,omitzero"`
}

// GatewayInfrastructure mirrors gateway.networking.k8s.io/v1.GatewayInfrastructure.
type GatewayInfrastructure struct {
	// Annotations applied by the gateway-controller to resources it creates
	// for this Gateway (e.g. the generated Service).
	Annotations map[string]string `json:"annotations,omitzero"`
	// Labels applied by the gateway-controller to resources it creates
	// for this Gateway.
	Labels map[string]string `json:"labels,omitzero"`
	// ParametersRef references an implementation-specific resource (typically
	// a ConfigMap) that customizes the generated infrastructure resources.
	ParametersRef *GatewayParametersRef `json:"parametersRef,omitzero"`
}

// GatewayParametersRef mirrors gateway.networking.k8s.io/v1.LocalParametersReference.
type GatewayParametersRef struct {
	// Group of the referent. Use "" for the core API.
	Group string `json:"group"`
	// Kind of the referent (e.g. "ConfigMap").
	Kind string `json:"kind"`
	// Name of the referent.
	Name string `json:"name"`
}

// HTTPRoutesConf holds configuration for HTTPRoute support
type HTTPRoutesConf struct {
	Enabled bool `json:"enabled,omitzero"`
}

// GatewayListener mirrors gateway.networking.k8s.io/v1.Listener
type GatewayListener struct {
	Name          string             `json:"name"`
	Hostname      string             `json:"hostname,omitzero"`
	Port          int32              `json:"port"`
	Protocol      string             `json:"protocol"`
	TLS           *ListenerTLSConfig `json:"tls,omitzero"`
	AllowedRoutes *AllowedRoutes     `json:"allowedRoutes,omitzero"`
}

// ListenerTLSConfig mirrors gateway.networking.k8s.io/v1.ListenerTLSConfig
type ListenerTLSConfig struct {
	Mode            string                  `json:"mode,omitzero"`
	CertificateRefs []SecretObjectReference `json:"certificateRefs,omitzero"`
	Options         map[string]string       `json:"options,omitzero"`
}

// SecretObjectReference mirrors gateway.networking.k8s.io/v1.SecretObjectReference
type SecretObjectReference struct {
	Group     string `json:"group,omitzero"`
	Kind      string `json:"kind,omitzero"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitzero"`
}

// AllowedRoutes mirrors gateway.networking.k8s.io/v1.AllowedRoutes
type AllowedRoutes struct {
	Namespaces *RouteNamespaces `json:"namespaces,omitzero"`
	Kinds      []RouteGroupKind `json:"kinds,omitzero"`
}

// RouteNamespaces mirrors gateway.networking.k8s.io/v1.RouteNamespaces
type RouteNamespaces struct {
	From     string                `json:"from,omitzero"` // "All", "Same", or "Selector"
	Selector *metav1.LabelSelector `json:"selector,omitzero"`
}

// RouteGroupKind mirrors gateway.networking.k8s.io/v1.RouteGroupKind
type RouteGroupKind struct {
	Group string `json:"group,omitzero"`
	Kind  string `json:"kind"`
}

// ShallCreateHTTPRoute returns true if the target workload shall create an HTTPRoute
func (c *OIDCAppsControllerConfig) ShallCreateHTTPRoute(object client.Object) bool {
	if !c.IsHTTPRouteEnabled() {
		return false
	}

	t := c.FetchTarget(object)
	if t.HTTPRoute != nil {
		if t.HTTPRoute.Create && len(t.HTTPRoute.ParentRefs) == 0 && !c.IsManagedGatewayEnabled() {
			c.log.Info("WARNING: HTTPRoute configuration has empty parentRefs - HTTPRoute will not be attached to any Gateway",
				"target", t.Name,
				"object", object.GetNamespace()+"/"+object.GetName())
		}

		return t.HTTPRoute.Create
	}

	return false
}

// IsHTTPRouteEnabled returns true if Gateway API HTTPRoute support is globally enabled
func (c *OIDCAppsControllerConfig) IsHTTPRouteEnabled() bool {
	return c.Global.Gateway != nil &&
		c.Global.Gateway.HTTPRoutes != nil &&
		c.Global.Gateway.HTTPRoutes.Enabled
}

// IsManagedGatewayEnabled returns true if the controller should create and manage a Gateway resource
func (c *OIDCAppsControllerConfig) IsManagedGatewayEnabled() bool {
	return c.IsHTTPRouteEnabled() && c.Global.Gateway.Managed
}

// GetManagedGatewayConf returns the Gateway configuration, or nil if not configured
func (c *OIDCAppsControllerConfig) GetManagedGatewayConf() *GatewayGlobalConf {
	return c.Global.Gateway
}

// GetHTTPRouteHost returns the host for the HTTPRoute for a given workload target
func (c *OIDCAppsControllerConfig) GetHTTPRouteHost(object client.Object) string {
	t := c.FetchTarget(object)
	domain := c.Global.DomainName

	prefix := object.GetName() + "-" + object.GetNamespace()
	if t.HTTPRoute != nil && t.HTTPRoute.HostPrefix != "" {
		prefix = t.HTTPRoute.HostPrefix + "-" + randutils.GenerateSha256(object.GetName()+"-"+object.GetNamespace())
	}

	if t.HTTPRoute != nil && t.HTTPRoute.Host != "" {
		prefix, domain, _ = strings.Cut(t.HTTPRoute.Host, ".")
	}

	if domain == "" {
		return prefix
	}

	return strings.Join([]string{prefix, domain}, ".")
}

// GetHTTPRouteParentRefs returns the parent references for the HTTPRoute for the given target.
// When the target has no parentRefs and a managed gateway is enabled, returns a default
// reference to the managed Gateway.
func (c *OIDCAppsControllerConfig) GetHTTPRouteParentRefs(object client.Object) []HTTPRouteParentRef {
	t := c.FetchTarget(object)
	if t.HTTPRoute != nil && len(t.HTTPRoute.ParentRefs) > 0 {
		return t.HTTPRoute.ParentRefs
	}

	if c.IsManagedGatewayEnabled() {
		name := c.Global.Gateway.Name
		if name == "" {
			name = constants.ManagedGatewayName
		}

		return []HTTPRouteParentRef{
			{
				Name:      name,
				Namespace: os.Getenv(constants.NAMESPACE),
			},
		}
	}

	return nil
}

// GetHTTPRouteAnnotations returns the HTTPRoute annotations for the given target
func (c *OIDCAppsControllerConfig) GetHTTPRouteAnnotations(object client.Object) map[string]string {
	t := c.FetchTarget(object)
	if t.HTTPRoute != nil && t.HTTPRoute.Annotations != nil {
		return t.HTTPRoute.Annotations
	}

	return nil
}

// GetHTTPRouteLabels returns the HTTPRoute labels for the given target
func (c *OIDCAppsControllerConfig) GetHTTPRouteLabels(object client.Object) map[string]string {
	t := c.FetchTarget(object)
	if t.HTTPRoute != nil && t.HTTPRoute.Labels != nil {
		return t.HTTPRoute.Labels
	}

	return nil
}

// GetHTTPRouteDefaultPath returns the default redirect path for the HTTPRoute of the given target.
// The path must start with "/" to be valid; invalid paths are ignored.
func (c *OIDCAppsControllerConfig) GetHTTPRouteDefaultPath(object client.Object) string {
	t := c.FetchTarget(object)
	if t.HTTPRoute != nil && isValidDefaultPath(t.HTTPRoute.DefaultPath) {
		return t.HTTPRoute.DefaultPath
	}

	return ""
}
