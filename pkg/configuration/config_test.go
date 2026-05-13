// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	_ "embed"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

//go:embed test/configuration.yaml
var configYaml string

func TestTargetMatchLabels(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create a fake client
	target := getDeployment("test-02")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target).
		WithObjects(getNginxDeployment()).
		Build()

	g.Expect(extensionConfig.Match(target)).To(BeTrue())
	g.Expect(extensionConfig.Match(getNginxDeployment())).To(BeFalse())
}

func TestTargetIngressHostPrefix(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create a fake client
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(getDeployment("test-02")).
		Build()

	z := strings.SplitN(extensionConfig.GetHost(getDeployment("test-02")), ".", 2)
	g.Expect(len(z)).To(Equal(2))
	g.Expect(z[0]).To(HavePrefix("test-02-prefix-"))
	g.Expect(z[1]).To(Equal("domain.org"))
}

func TestTargetIngressHost(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create a fake client
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(getDeployment("test-03")).
		Build()

	g.Expect(extensionConfig.GetHost(getDeployment("test-03"))).To(Equal("this.overwrites"))
}

func TestTargetWithoutIngressHost(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create a fake client
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(getDeployment("test-04")).
		Build()

	g.Expect(extensionConfig.GetHost(getDeployment("test-04"))).To(Equal("test-04-test.domain.org"))
}

func TestTargetGlobalKubeSecret(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create a fake client
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(getDeployment("test-03")).
		Build()

	g.Expect(extensionConfig.GetKubeSecretName(getDeployment("test-03"))).To(Equal("kubeconfig"))
	g.Expect(extensionConfig.ShallCreateIngress(getDeployment("test-03"))).To(BeFalse())
}

func TestTargetKubeSecret(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create a fake client
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(getDeployment("test-02")).
		Build()

	g.Expect(extensionConfig.ShallCreateIngress(getDeployment("test-02"))).To(BeTrue())
	g.Expect(extensionConfig.GetKubeSecretName(getDeployment("test-02"))).To(Equal("target-kubeconfig"))
}

func TestTargetGlobalConfiguration(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create a fake client
	target := getDeployment("test-04")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target).
		Build()

	g.Expect(extensionConfig.GetClientID(target)).To(Equal("client-id"))
	g.Expect(extensionConfig.GetScope(target)).To(Equal("openid email"))
	g.Expect(extensionConfig.GetClientSecret(target)).To(Equal("client-secret"))
	g.Expect(extensionConfig.GetRedirectURL(target)).To(Equal("https://test-04-test.domain.org/oauth2/callback"))
	g.Expect(extensionConfig.GetOidcIssuerURL(target)).To(Equal("https://oidc-provider.org"))
	g.Expect(extensionConfig.GetSslInsecureSkipVerify(target)).To(BeFalse())
	g.Expect(extensionConfig.GetInsecureOidcSkipIssuerVerification(target)).To(BeFalse())
	g.Expect(extensionConfig.GetInsecureOidcSkipNonce(target)).To(BeFalse())
	g.Expect(extensionConfig.GetKubeConfigStr(target)).To(Equal("Imt1YmVjb25maWci"))
	g.Expect(extensionConfig.GetKubeSecretName(target)).To(Equal("kubeconfig"))
	g.Expect(extensionConfig.ShallCreateIngress(target)).To(BeFalse())
}

func TestTargetConfiguration(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create a fake client
	target := getDeployment("test-02")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target).
		Build()

	g.Expect(extensionConfig.GetClientID(target)).To(Equal("client-id-target"))
	g.Expect(extensionConfig.GetScope(target)).To(Equal("openid email target"))
	g.Expect(extensionConfig.GetClientSecret(target)).To(Equal("client-secret-target"))
	g.Expect(extensionConfig.GetRedirectURL(target)).To(Equal("https://app.org/oauth2/callback"))
	g.Expect(extensionConfig.GetOidcIssuerURL(target)).To(Equal("https://oidc-provider-target.org"))
	g.Expect(extensionConfig.GetOidcCASecretName(target)).To(Equal("target-oidc-ca"))
	g.Expect(extensionConfig.GetSslInsecureSkipVerify(target)).To(BeTrue())
	g.Expect(extensionConfig.GetInsecureOidcSkipIssuerVerification(target)).To(BeTrue())
	g.Expect(extensionConfig.GetInsecureOidcSkipNonce(target)).To(BeTrue())
	g.Expect(extensionConfig.GetKubeConfigStr(target)).To(Equal("a3ViZWNvbmZpZy10YXJnZXQK"))
	g.Expect(extensionConfig.GetKubeSecretName(target)).To(Equal("target-kubeconfig"))
}

func TestHTTPRouteConfiguration(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{
		Global: Global{Gateway: &GatewayGlobalConf{HTTPRoutes: &HTTPRoutesConf{Enabled: true}}},
	}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create a fake client
	target := getDeployment("test-05")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target).
		Build()

	g.Expect(extensionConfig.ShallCreateHTTPRoute(target)).To(BeTrue())

	parentRefs := extensionConfig.GetHTTPRouteParentRefs(target)
	g.Expect(parentRefs).To(HaveLen(1))
	g.Expect(parentRefs[0].Name).To(Equal("my-gateway"))
	g.Expect(parentRefs[0].Namespace).To(Equal("gateway-system"))
	g.Expect(parentRefs[0].SectionName).To(Equal("https"))

	host := extensionConfig.GetHTTPRouteHost(target)
	g.Expect(host).To(HavePrefix("test-05-prefix-"))
	g.Expect(host).To(HaveSuffix(".domain.org"))
}

func TestHTTPRouteWithCustomHost(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{
		Global: Global{Gateway: &GatewayGlobalConf{HTTPRoutes: &HTTPRoutesConf{Enabled: true}}},
	}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create a fake client
	target := getDeployment("test-06")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target).
		Build()

	g.Expect(extensionConfig.ShallCreateHTTPRoute(target)).To(BeTrue())
	g.Expect(extensionConfig.GetHTTPRouteHost(target)).To(Equal("custom.httproute.host"))

	parentRefs := extensionConfig.GetHTTPRouteParentRefs(target)
	g.Expect(parentRefs).To(HaveLen(1))
	g.Expect(parentRefs[0].Name).To(Equal("another-gateway"))
	g.Expect(parentRefs[0].Namespace).To(Equal(""))
	g.Expect(parentRefs[0].SectionName).To(Equal(""))
}

func TestTargetWithoutHTTPRoute(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{
		Global: Global{Gateway: &GatewayGlobalConf{HTTPRoutes: &HTTPRoutesConf{Enabled: true}}},
	}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create a fake client
	target := getDeployment("test-04")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target).
		Build()

	g.Expect(extensionConfig.ShallCreateHTTPRoute(target)).To(BeFalse())
	g.Expect(extensionConfig.GetHTTPRouteParentRefs(target)).To(BeNil())
}

func TestHTTPRouteDisabledGlobally(t *testing.T) {
	// Even with HTTPRoute configured in target, it should return false when global.gateway.httpRoutes.enabled is false
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create a fake client - test-05 has HTTPRoute configured with create: true
	target := getDeployment("test-05")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target).
		Build()

	// Should return false because global.gateway.httpRoutes.enabled is not set (nil)
	g.Expect(extensionConfig.ShallCreateHTTPRoute(target)).To(BeFalse())
	g.Expect(extensionConfig.IsHTTPRouteEnabled()).To(BeFalse())
}

func TestIsHTTPRouteEnabled(t *testing.T) {
	g := NewWithT(t)

	// Test nil Gateway
	configNil := &OIDCAppsControllerConfig{}
	g.Expect(configNil.IsHTTPRouteEnabled()).To(BeFalse())

	// Test HTTPRoutes with enabled=false
	configDisabled := &OIDCAppsControllerConfig{
		Global: Global{Gateway: &GatewayGlobalConf{HTTPRoutes: &HTTPRoutesConf{Enabled: false}}},
	}
	g.Expect(configDisabled.IsHTTPRouteEnabled()).To(BeFalse())

	// Test HTTPRoutes with enabled=true
	configEnabled := &OIDCAppsControllerConfig{
		Global: Global{Gateway: &GatewayGlobalConf{HTTPRoutes: &HTTPRoutesConf{Enabled: true}}},
	}
	g.Expect(configEnabled.IsHTTPRouteEnabled()).To(BeTrue())
}

func TestHTTPRouteWithEmptyParentRefs(t *testing.T) {
	// Test that HTTPRoute with empty parentRefs still returns true for ShallCreateHTTPRoute
	// but logs a warning (the warning is tested implicitly by ensuring the function works)
	extensionConfig := OIDCAppsControllerConfig{
		Global: Global{Gateway: &GatewayGlobalConf{HTTPRoutes: &HTTPRoutesConf{Enabled: true}}},
	}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create a fake client - test-07 has HTTPRoute with create: true but no parentRefs
	target := getDeployment("test-07")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target).
		Build()

	// Should return true for create, but parentRefs should be empty
	g.Expect(extensionConfig.ShallCreateHTTPRoute(target)).To(BeTrue())
	g.Expect(extensionConfig.GetHTTPRouteParentRefs(target)).To(BeNil())

	// Verify the host is still generated correctly
	host := extensionConfig.GetHTTPRouteHost(target)
	g.Expect(host).To(HavePrefix("test-07-prefix-"))
	g.Expect(host).To(HaveSuffix(".domain.org"))
}

func getDeployment(name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
			Labels:    map[string]string{"app.kubernetes.io/name": name},
		},
	}
}

func getNginxDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "default",
			Labels:    map[string]string{"app.kubernetes.io/name": "nginx"},
		},
	}
}

func getTestNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test",
			Labels: map[string]string{"kubernetes.io/metadata.name": "test"},
		},
	}
}

func TestGetIngressDefaultPath(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(getDeployment("test-08")).
		WithObjects(getDeployment("test-02")).
		Build()

	g.Expect(extensionConfig.GetIngressDefaultPath(getDeployment("test-08"))).To(Equal("/select/vmui"))
	g.Expect(extensionConfig.GetIngressDefaultPath(getDeployment("test-02"))).To(BeEmpty())
}

func TestGetHTTPRouteDefaultPath(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{
		Global: Global{Gateway: &GatewayGlobalConf{HTTPRoutes: &HTTPRoutesConf{Enabled: true}}},
	}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(getDeployment("test-09")).
		WithObjects(getDeployment("test-05")).
		Build()

	g.Expect(extensionConfig.GetHTTPRouteDefaultPath(getDeployment("test-09"))).To(Equal("/dashboard"))
	g.Expect(extensionConfig.GetHTTPRouteDefaultPath(getDeployment("test-05"))).To(BeEmpty())
}

func TestGetUpstreamTarget(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(getDeployment("test-02")).
		WithObjects(getDeployment("test-04")).
		Build()

	// test-02 has targetProtocol: https and targetPort: 8443
	g.Expect(extensionConfig.GetUpstreamTarget(getDeployment("test-02"))).To(Equal("protocol=https, port=8443"))

	// test-04 has no targetPort/targetProtocol defined — defaults to http and 0
	g.Expect(extensionConfig.GetUpstreamTarget(getDeployment("test-04"))).To(Equal("protocol=http, port=0"))
}

func TestGetOidcCABundle(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(getDeployment("test-04")).
		WithObjects(getDeployment("test-12")).
		Build()

	// test-04 uses global oidcCABundle
	bundle := extensionConfig.GetOidcCABundle(getDeployment("test-04"))
	g.Expect(bundle).To(Equal("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n"))

	// test-12 has target-level oidcCABundle
	bundle = extensionConfig.GetOidcCABundle(getDeployment("test-12"))
	g.Expect(bundle).To(Equal("target-ca-bundle"))
}

func TestIsManagedGatewayEnabled(t *testing.T) {
	g := NewWithT(t)

	// Not enabled when gateway is nil
	configNil := &OIDCAppsControllerConfig{}
	g.Expect(configNil.IsManagedGatewayEnabled()).To(BeFalse())

	// Not enabled when HTTPRoutes disabled
	configNoHTTP := &OIDCAppsControllerConfig{
		Global: Global{Gateway: &GatewayGlobalConf{Managed: true}},
	}
	g.Expect(configNoHTTP.IsManagedGatewayEnabled()).To(BeFalse())

	// Enabled when HTTPRoutes enabled AND managed is true
	configManaged := &OIDCAppsControllerConfig{
		Global: Global{Gateway: &GatewayGlobalConf{
			HTTPRoutes:       &HTTPRoutesConf{Enabled: true},
			Managed:          true,
			GatewayClassName: "envoy",
		}},
	}
	g.Expect(configManaged.IsManagedGatewayEnabled()).To(BeTrue())

	// Not enabled when HTTPRoutes enabled but managed is false
	configNotManaged := &OIDCAppsControllerConfig{
		Global: Global{Gateway: &GatewayGlobalConf{
			HTTPRoutes: &HTTPRoutesConf{Enabled: true},
			Managed:    false,
		}},
	}
	g.Expect(configNotManaged.IsManagedGatewayEnabled()).To(BeFalse())
}

func TestGetManagedGatewayConf(t *testing.T) {
	g := NewWithT(t)

	configNil := &OIDCAppsControllerConfig{}
	g.Expect(configNil.GetManagedGatewayConf()).To(BeNil())

	gw := &GatewayGlobalConf{
		Managed:          true,
		Name:             "my-gw",
		GatewayClassName: "envoy",
	}
	configWithGW := &OIDCAppsControllerConfig{
		Global: Global{Gateway: gw},
	}
	g.Expect(configWithGW.GetManagedGatewayConf()).To(Equal(gw))
}

func TestGetHTTPRouteParentRefsManagedGatewayFallback(t *testing.T) {
	g := NewWithT(t)
	extensionConfig := OIDCAppsControllerConfig{
		Global: Global{Gateway: &GatewayGlobalConf{
			HTTPRoutes: &HTTPRoutesConf{Enabled: true},
			Managed:    true,
			Name:       "managed-gw",
		}},
	}
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// test-11 has HTTPRoute with no parentRefs — should fall back to managed gateway
	target := getDeployment("test-11")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target).
		Build()

	parentRefs := extensionConfig.GetHTTPRouteParentRefs(target)
	g.Expect(parentRefs).To(HaveLen(1))
	g.Expect(parentRefs[0].Name).To(Equal("managed-gw"))
}

func TestGetHTTPRouteAnnotationsAndLabels(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{
		Global: Global{Gateway: &GatewayGlobalConf{HTTPRoutes: &HTTPRoutesConf{Enabled: true}}},
	}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// test-11 has HTTPRoute annotations and labels
	target := getDeployment("test-11")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target).
		Build()

	annotations := extensionConfig.GetHTTPRouteAnnotations(target)
	g.Expect(annotations).To(HaveKeyWithValue("gateway.networking.k8s.io/purpose", "testing"))

	lbls := extensionConfig.GetHTTPRouteLabels(target)
	g.Expect(lbls).To(HaveKeyWithValue("route-type", "managed"))

	// test-05 has no annotations or labels on HTTPRoute
	target05 := getDeployment("test-05")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target05).
		Build()

	g.Expect(extensionConfig.GetHTTPRouteAnnotations(target05)).To(BeNil())
	g.Expect(extensionConfig.GetHTTPRouteLabels(target05)).To(BeNil())
}

func TestGetIngressAnnotationsAndLabels(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// test-10 has ingress annotations and labels
	target := getDeployment("test-10")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target).
		Build()

	annotations := extensionConfig.GetIngressAnnotations(target)
	g.Expect(annotations).To(HaveLen(2))
	g.Expect(annotations).To(HaveKeyWithValue("nginx.ingress.kubernetes.io/rewrite-target", "/"))
	g.Expect(annotations).To(HaveKeyWithValue("cert-manager.io/cluster-issuer", "letsencrypt"))

	lbls := extensionConfig.GetIngressLabels(target)
	g.Expect(lbls).To(HaveLen(2))
	g.Expect(lbls).To(HaveKeyWithValue("app.kubernetes.io/component", "ingress"))
	g.Expect(lbls).To(HaveKeyWithValue("environment", "production"))

	// test-04 has no ingress — should return nil
	target04 := getDeployment("test-04")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target04).
		Build()

	g.Expect(extensionConfig.GetIngressAnnotations(target04)).To(BeNil())
	g.Expect(extensionConfig.GetIngressLabels(target04)).To(BeNil())
}

func TestGetIngressClassName(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(getDeployment("test-10")).
		WithObjects(getDeployment("test-02")).
		Build()

	g.Expect(extensionConfig.GetIngressClassName(getDeployment("test-10"))).To(Equal("nginx"))
	g.Expect(extensionConfig.GetIngressClassName(getDeployment("test-02"))).To(BeEmpty())
}

func TestGetRedirectURLWithHTTPRoute(t *testing.T) {
	extensionConfig := OIDCAppsControllerConfig{
		Global: Global{Gateway: &GatewayGlobalConf{HTTPRoutes: &HTTPRoutesConf{Enabled: true}}},
	}
	g := NewWithT(t)
	err := yaml.Unmarshal([]byte(configYaml), &extensionConfig)
	g.Expect(err).ShouldNot(HaveOccurred())

	// test-06 has httpRoute with host "custom.httproute.host" but no explicit redirectUrl
	target := getDeployment("test-06")
	extensionConfig.client = fake.NewClientBuilder().
		WithObjects(getTestNamespace()).
		WithObjects(target).
		Build()

	// GetRedirectURL falls through to https://{GetHost}/oauth2/callback
	// GetHost uses ingress.Host (not httpRoute.Host), so it falls back to default
	redirectURL := extensionConfig.GetRedirectURL(target)
	g.Expect(redirectURL).To(Equal("https://test-06-test.domain.org/oauth2/callback"))
}

func TestIsValidDefaultPath(t *testing.T) {
	g := NewWithT(t)

	// Valid paths
	g.Expect(isValidDefaultPath("/dashboard")).To(BeTrue())
	g.Expect(isValidDefaultPath("/select/vmui")).To(BeTrue())
	g.Expect(isValidDefaultPath("/my-app/v2")).To(BeTrue())
	g.Expect(isValidDefaultPath("/path_with.dots~tilde")).To(BeTrue())

	// Invalid: empty, root only, missing leading slash
	g.Expect(isValidDefaultPath("")).To(BeFalse())
	g.Expect(isValidDefaultPath("/")).To(BeFalse())
	g.Expect(isValidDefaultPath("no-leading-slash")).To(BeFalse())

	// Invalid: disallowed characters
	g.Expect(isValidDefaultPath("/path?query=1")).To(BeFalse())
	g.Expect(isValidDefaultPath("/path#anchor")).To(BeFalse())
	g.Expect(isValidDefaultPath("/path;inject")).To(BeFalse())
	g.Expect(isValidDefaultPath("/path with spaces")).To(BeFalse())

	// Exactly maxDefaultPathLength characters: valid
	pathAtLimit := "/" + strings.Repeat("a", maxDefaultPathLength-1)
	g.Expect(len(pathAtLimit)).To(Equal(maxDefaultPathLength))
	g.Expect(isValidDefaultPath(pathAtLimit)).To(BeTrue())

	// maxDefaultPathLength + 1 characters: exceeds limit
	pathOverLimit := "/" + strings.Repeat("a", maxDefaultPathLength)
	g.Expect(len(pathOverLimit)).To(Equal(maxDefaultPathLength + 1))
	g.Expect(isValidDefaultPath(pathOverLimit)).To(BeFalse())
}
