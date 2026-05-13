// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OIDCAppsControllerConfig is the root configuration node
type OIDCAppsControllerConfig struct {
	Global  Global   `json:"global"`
	Targets []Target `json:"targets,omitzero"`
	client  client.Client
	log     logr.Logger
}

// Global holds the concrete target configurations for the auth & authz proxies
type Global struct {
	Oauth2Proxy   *Oauth2ProxyConfig   `json:"oauth2Proxy,omitzero"`
	KubeRbacProxy *KubeRbacProxyConfig `json:"kubeRbacProxy,omitzero"`

	Labels      map[string]string `json:"labels,omitzero"`
	Annotations map[string]string `json:"annotations,omitzero"`
	DomainName  string            `json:"domainName,omitzero"`

	OidcCABundle    string                  `json:"oidcCABundle,omitzero"`
	OidcCASecretRef *corev1.SecretReference `json:"oidcCASecretRef,omitzero"`

	// Gateway holds global configuration for Gateway API support
	Gateway *GatewayGlobalConf `json:"gateway,omitzero"`
}

// Oauth2ProxyConfig OIDC Provider configuration
type Oauth2ProxyConfig struct {
	Scope                              string `json:"scope,omitzero"`
	ClientID                           string `json:"clientId"`
	ClientSecret                       string `json:"clientSecret,omitzero"` // #nosec G117
	RedirectURL                        string `json:"redirectUrl"`
	OidcIssuerURL                      string `json:"oidcIssuerUrl"`
	SSLInsecureSkipVerify              *bool  `json:"sslInsecureSkipVerify,omitzero"`
	InsecureOidcSkipIssuerVerification *bool  `json:"insecureOidcSkipIssuerVerification,omitzero"`
	InsecureOidcSkipNonce              *bool  `json:"insecureOidcSkipNonce,omitzero"`
}

// KubeRbacProxyConfig kube-rbac-proxy configuration
type KubeRbacProxyConfig struct {
	KubeConfigStr string                  `json:"kubeConfigStr,omitzero"`
	KubeSecretRef *corev1.SecretReference `json:"kubeSecretRef,omitzero"`
}

// Target workload selector configuration
type Target struct {
	Name              string                `json:"name"`
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitzero"`
	LabelSelector     *metav1.LabelSelector `json:"labelSelector,omitzero"`
	TargetPort        intstr.IntOrString    `json:"targetPort"`
	TargetProtocol    string                `json:"targetProtocol,omitzero"`
	Ingress           *IngressConf          `json:"ingress,omitzero"`
	HTTPRoute         *HTTPRouteConf        `json:"httpRoute,omitzero"`
	Global
}

// IngressConf holds configuration for the ingress entry-point
type IngressConf struct {
	Create           bool                   `json:"create,omitzero"`
	HostPrefix       string                 `json:"hostPrefix,omitzero"`
	Host             string                 `json:"host,omitzero"`
	Annotations      map[string]string      `json:"annotations,omitzero"`
	Labels           map[string]string      `json:"labels,omitzero"`
	TLSSecretRef     corev1.SecretReference `json:"tlsSecretRef,omitzero"`
	IngressClassName string                 `json:"ingressClassName,omitzero"`
	DefaultPath      string                 `json:"defaultPath,omitzero"`
}

// HTTPRouteConf holds configuration for the Gateway API HTTPRoute entry-point
type HTTPRouteConf struct {
	Create      bool                 `json:"create,omitzero"`
	HostPrefix  string               `json:"hostPrefix,omitzero"`
	Host        string               `json:"host,omitzero"`
	Annotations map[string]string    `json:"annotations,omitzero"`
	Labels      map[string]string    `json:"labels,omitzero"`
	ParentRefs  []HTTPRouteParentRef `json:"parentRefs,omitzero"`
	DefaultPath string               `json:"defaultPath,omitzero"`
}

// HTTPRouteParentRef defines a reference to a parent Gateway
type HTTPRouteParentRef struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace,omitzero"`
	SectionName string `json:"sectionName,omitzero"`
}
