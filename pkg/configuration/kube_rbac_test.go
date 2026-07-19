// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestGetKubeConfigStrNilKubeRbacProxy verifies that GetKubeConfigStr returns "" when
// Global.KubeRbacProxy is nil — the expected production configuration that causes
// kube-rbac-proxy to use the pod's service account token for SubjectAccessReviews
// rather than a Dex OIDC token.
func TestGetKubeConfigStrNilKubeRbacProxy(t *testing.T) {
	g := NewWithT(t)

	cfg := &OIDCAppsControllerConfig{
		Global: Global{
			KubeRbacProxy: nil,
		},
		Targets: []Target{
			{Name: "test-target"},
		},
	}
	cfg.client = fake.NewClientBuilder().Build()

	target := getDeployment("test-target")
	g.Expect(cfg.GetKubeConfigStr(target)).To(Equal(""),
		"GetKubeConfigStr should return empty string when KubeRbacProxy is nil")
}

// TestGetKubeConfigStrEmptyKubeConfigStr verifies that GetKubeConfigStr returns "" when
// KubeRbacProxy is set but KubeConfigStr is explicitly empty.
func TestGetKubeConfigStrEmptyKubeConfigStr(t *testing.T) {
	g := NewWithT(t)

	cfg := &OIDCAppsControllerConfig{
		Global: Global{
			KubeRbacProxy: &KubeRbacProxyConfig{
				KubeConfigStr: "",
				KubeSecretRef: nil,
			},
		},
	}
	cfg.client = fake.NewClientBuilder().Build()

	target := getDeployment("any")
	g.Expect(cfg.GetKubeConfigStr(target)).To(Equal(""),
		"GetKubeConfigStr should return empty string when KubeConfigStr is empty")
}

// TestGetKubeSecretNameNilKubeRbacProxy verifies that GetKubeSecretName returns "" when
// Global.KubeRbacProxy is nil.
func TestGetKubeSecretNameNilKubeRbacProxy(t *testing.T) {
	g := NewWithT(t)

	cfg := &OIDCAppsControllerConfig{
		Global: Global{
			KubeRbacProxy: nil,
		},
	}
	cfg.client = fake.NewClientBuilder().Build()

	target := getDeployment("any")
	g.Expect(cfg.GetKubeSecretName(target)).To(Equal(""),
		"GetKubeSecretName should return empty string when KubeRbacProxy is nil")
}

// TestGetKubeSecretNameEmptyRef verifies that GetKubeSecretName returns "" when
// KubeSecretRef is nil or has an empty name.
func TestGetKubeSecretNameEmptyRef(t *testing.T) {
	g := NewWithT(t)

	cfg := &OIDCAppsControllerConfig{
		Global: Global{
			KubeRbacProxy: &KubeRbacProxyConfig{
				KubeSecretRef: nil,
			},
		},
	}
	cfg.client = fake.NewClientBuilder().Build()

	target := getDeployment("any")
	g.Expect(cfg.GetKubeSecretName(target)).To(Equal(""),
		"GetKubeSecretName should return empty string when KubeSecretRef is nil")

	cfg.Global.KubeRbacProxy.KubeSecretRef = &corev1.SecretReference{Name: ""}
	g.Expect(cfg.GetKubeSecretName(target)).To(Equal(""),
		"GetKubeSecretName should return empty string when KubeSecretRef.Name is empty")
}

// TestGetKubeConfigStrTargetOverridesGlobal verifies that a per-target KubeConfigStr
// takes precedence over the global value.
func TestGetKubeConfigStrTargetOverridesGlobal(t *testing.T) {
	g := NewWithT(t)

	cfg := &OIDCAppsControllerConfig{
		Global: Global{
			KubeRbacProxy: &KubeRbacProxyConfig{
				KubeConfigStr: "global-kubeconfig",
			},
		},
		Targets: []Target{
			{
				Name: "target-with-override",
				Global: Global{
					KubeRbacProxy: &KubeRbacProxyConfig{
						KubeConfigStr: "target-kubeconfig",
					},
				},
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app.kubernetes.io/name": "target-with-override"},
				},
			},
		},
	}
	cfg.client = fake.NewClientBuilder().Build()

	target := getDeployment("target-with-override")
	g.Expect(cfg.GetKubeConfigStr(target)).To(Equal("target-kubeconfig"),
		"per-target KubeConfigStr should take precedence over global")
}

// TestGetKubeConfigStrGlobalFallback verifies that when a target has no KubeRbacProxy
// config, GetKubeConfigStr falls back to the global value.
func TestGetKubeConfigStrGlobalFallback(t *testing.T) {
	g := NewWithT(t)

	cfg := &OIDCAppsControllerConfig{
		Global: Global{
			KubeRbacProxy: &KubeRbacProxyConfig{
				KubeConfigStr: "global-kubeconfig",
			},
		},
		Targets: []Target{
			{
				Name: "target-no-override",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app.kubernetes.io/name": "target-no-override"},
				},
			},
		},
	}
	cfg.client = fake.NewClientBuilder().Build()

	target := getDeployment("target-no-override")
	g.Expect(cfg.GetKubeConfigStr(target)).To(Equal("global-kubeconfig"),
		"GetKubeConfigStr should fall back to global when target has no KubeRbacProxy config")
}

// TestGetKubeSecretNameTargetOverridesGlobal verifies per-target KubeSecretRef precedence.
func TestGetKubeSecretNameTargetOverridesGlobal(t *testing.T) {
	g := NewWithT(t)

	cfg := &OIDCAppsControllerConfig{
		Global: Global{
			KubeRbacProxy: &KubeRbacProxyConfig{
				KubeSecretRef: &corev1.SecretReference{Name: "global-secret"},
			},
		},
		Targets: []Target{
			{
				Name: "target-with-secret",
				Global: Global{
					KubeRbacProxy: &KubeRbacProxyConfig{
						KubeSecretRef: &corev1.SecretReference{Name: "target-secret"},
					},
				},
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app.kubernetes.io/name": "target-with-secret"},
				},
			},
		},
	}
	cfg.client = fake.NewClientBuilder().Build()

	target := getDeployment("target-with-secret")
	g.Expect(cfg.GetKubeSecretName(target)).To(Equal("target-secret"),
		"per-target KubeSecretRef should take precedence over global")
}
