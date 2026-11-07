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

// TestGetClientSecretNameNilOauth2Proxy verifies the accessor returns "" when Oauth2Proxy is nil.
func TestGetClientSecretNameNilOauth2Proxy(t *testing.T) {
	g := NewWithT(t)

	cfg := &OIDCAppsControllerConfig{
		Global: Global{Oauth2Proxy: nil},
	}
	cfg.client = fake.NewClientBuilder().Build()

	target := getDeployment("any")
	g.Expect(cfg.Resolve(t.Context(), target).GetClientSecretName()).To(Equal(""),
		"GetClientSecretName should return empty string when Oauth2Proxy is nil")
}

// TestGetClientSecretNameEmptyRef verifies the accessor returns "" when ClientSecretRef is nil or empty.
func TestGetClientSecretNameEmptyRef(t *testing.T) {
	g := NewWithT(t)

	cfg := &OIDCAppsControllerConfig{
		Global: Global{Oauth2Proxy: &Oauth2ProxyConfig{ClientSecretRef: nil}},
	}
	cfg.client = fake.NewClientBuilder().Build()

	target := getDeployment("any")
	g.Expect(cfg.Resolve(t.Context(), target).GetClientSecretName()).To(Equal(""),
		"GetClientSecretName should return empty string when ClientSecretRef is nil")

	cfg.Global.Oauth2Proxy.ClientSecretRef = &corev1.SecretReference{Name: ""}
	g.Expect(cfg.Resolve(t.Context(), target).GetClientSecretName()).To(Equal(""),
		"GetClientSecretName should return empty string when ClientSecretRef.Name is empty")
}

// TestGetClientSecretNameGlobalFallback verifies the global ref is used when no target-level ref is set.
func TestGetClientSecretNameGlobalFallback(t *testing.T) {
	g := NewWithT(t)

	cfg := &OIDCAppsControllerConfig{
		Global: Global{
			Oauth2Proxy: &Oauth2ProxyConfig{
				ClientSecretRef: &corev1.SecretReference{Name: "global-client-secret"},
			},
		},
		Targets: []Target{
			{
				Name: "test-target",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app.kubernetes.io/name": "test-target"},
				},
			},
		},
	}
	cfg.client = fake.NewClientBuilder().Build()

	target := getDeployment("test-target")
	g.Expect(cfg.Resolve(t.Context(), target).GetClientSecretName()).To(Equal("global-client-secret"),
		"GetClientSecretName should fall back to the global ClientSecretRef")
}

// TestGetClientSecretNameTargetOverride verifies a target-level ref overrides the global one.
func TestGetClientSecretNameTargetOverride(t *testing.T) {
	g := NewWithT(t)

	cfg := &OIDCAppsControllerConfig{
		Global: Global{
			Oauth2Proxy: &Oauth2ProxyConfig{
				ClientSecretRef: &corev1.SecretReference{Name: "global-client-secret"},
			},
		},
		Targets: []Target{
			{
				Name: "test-target",
				Oauth2Proxy: &Oauth2ProxyConfig{
					ClientSecretRef: &corev1.SecretReference{Name: "target-client-secret"},
				},
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app.kubernetes.io/name": "test-target"},
				},
			},
		},
	}
	cfg.client = fake.NewClientBuilder().Build()

	target := getDeployment("test-target")
	g.Expect(cfg.Resolve(t.Context(), target).GetClientSecretName()).To(Equal("target-client-secret"),
		"GetClientSecretName should prefer the target-level ClientSecretRef")
}
