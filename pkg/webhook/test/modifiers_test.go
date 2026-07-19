// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	adminssionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
	"github.com/nickytd/oidc-apps/pkg/randutils"
	"github.com/nickytd/oidc-apps/pkg/webhook"
)

var _ = Describe("Cookie Secret Deterministic Generation Tests", func() {
	Context("when verifying deterministic cookie secret generation", func() {
		var (
			deployment1      *appsv1.Deployment
			replicaSet1      *appsv1.ReplicaSet
			pod1             *corev1.Pod
			localPodWebhook1 *webhook.PodMutator
		)

		BeforeEach(func() {
			deployment1 = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx",
					Namespace: "nginx",
					Labels:    map[string]string{"app": "nginx"},
					UID:       "deployment-uid-1",
				},
			}
			replicaSet1 = &appsv1.ReplicaSet{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-rs-0001",
					Namespace: "nginx",
					UID:       "replicaset-uid-1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "nginx",
							UID:        "deployment-uid-1",
						},
					},
				},
			}
			pod1 = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-pod-1",
					Namespace: "nginx",
					UID:       "pod-uid-1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "ReplicaSet",
							Name:       "nginx-rs-0001",
							UID:        "replicaset-uid-1",
						},
					},
				},
			}

			s := runtime.NewScheme()
			err := scheme.AddToScheme(s)
			Expect(err).NotTo(HaveOccurred())

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(deployment1, replicaSet1, pod1).
				Build()

			configuration.CreateControllerConfigOrDie(
				filepath.Join(tmpDir, "config.yaml"),
				configuration.WithClient(fakeClient),
				configuration.WithLog(_log),
			)

			localPodWebhook1 = &webhook.PodMutator{
				Client:  fakeClient,
				Decoder: admission.NewDecoder(s),
			}
		})

		It("should generate the same cookie secret for the same owner inputs", func() {
			// First call
			patchedPod1 := patchPodWithWebhook(pod1, localPodWebhook1)
			cookieSecret1 := extractCookieSecret(patchedPod1)

			// Second call with the same inputs
			patchedPod2 := patchPodWithWebhook(pod1, localPodWebhook1)
			cookieSecret2 := extractCookieSecret(patchedPod2)

			Expect(cookieSecret1).NotTo(BeEmpty(), "Cookie secret should not be empty")
			Expect(cookieSecret2).NotTo(BeEmpty(), "Cookie secret should not be empty")
			Expect(cookieSecret1).To(Equal(cookieSecret2), "Same inputs should produce same cookie secret")

			_log.Info("Deterministic cookie secret verified", "secret1", cookieSecret1, "secret2", cookieSecret2)
		})

		It("should generate the same cookie secret across multiple invocations", func() {
			const iterations = 5

			var cookieSecrets []string

			for range iterations {
				patchedPod := patchPodWithWebhook(pod1, localPodWebhook1)
				cookieSecret := extractCookieSecret(patchedPod)
				cookieSecrets = append(cookieSecrets, cookieSecret)
			}

			// Verify all secrets are identical
			for i := 1; i < iterations; i++ {
				Expect(cookieSecrets[i]).To(Equal(cookieSecrets[0]),
					"Cookie secret should be identical across all invocations")
			}

			_log.Info("Multiple invocations verified", "iterations", iterations, "secret", cookieSecrets[0])
		})

		It("should match the expected deterministic value based on owner name, namespace, and UID", func() {
			patchedPod := patchPodWithWebhook(pod1, localPodWebhook1)
			actualCookieSecret := extractCookieSecret(patchedPod)

			// The cookie secret is generated using: rand.GenerateFullSha256(...) stripped to 32 hex chars (16 bytes)
			// The owner is the Deployment, so: "nginx" + "-" + "nginx" + "-" + "deployment-uid-1" + "-cookie-secret"
			fullHash := randutils.GenerateFullSha256("nginx" + "-" + "nginx" + "-" + "deployment-uid-1" + "-cookie-secret")
			expectedCookieSecret := fullHash[:32]

			Expect(actualCookieSecret).To(Equal(expectedCookieSecret),
				"Cookie secret should match expected deterministic value")

			_log.Info("Expected value verified", "expected", expectedCookieSecret, "actual", actualCookieSecret)
		})

		It("should generate a cookie secret of exactly 32 hex characters (16 bytes)", func() {
			patchedPod := patchPodWithWebhook(pod1, localPodWebhook1)
			actualCookieSecret := extractCookieSecret(patchedPod)

			// Cookie secret must be 16 bytes (32 hex chars) for AES-128 cipher compatibility
			Expect(len(actualCookieSecret)).To(Equal(32),
				"Cookie secret should be exactly 32 hex characters (16 bytes)")

			_log.Info("Cookie secret length verified", "length", len(actualCookieSecret), "expectedLength", 32)
		})
	})

	Context("when different owners produce different secrets", func() {
		var (
			deployment1     *appsv1.Deployment
			replicaSet1     *appsv1.ReplicaSet
			pod1            *corev1.Pod
			deployment2     *appsv1.Deployment
			replicaSet2     *appsv1.ReplicaSet
			pod2            *corev1.Pod
			deployment3     *appsv1.Deployment
			replicaSet3     *appsv1.ReplicaSet
			pod3            *corev1.Pod
			localPodWebhook *webhook.PodMutator
		)

		BeforeEach(func() {
			// First owner: nginx in nginx namespace
			deployment1 = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx",
					Namespace: "nginx",
					Labels:    map[string]string{"app": "nginx"},
					UID:       "deployment-uid-1",
				},
			}
			replicaSet1 = &appsv1.ReplicaSet{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-rs-0001",
					Namespace: "nginx",
					UID:       "replicaset-uid-1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "nginx",
							UID:        "deployment-uid-1",
						},
					},
				},
			}
			pod1 = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-pod-1",
					Namespace: "nginx",
					UID:       "pod-uid-1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "ReplicaSet",
							Name:       "nginx-rs-0001",
							UID:        "replicaset-uid-1",
						},
					},
				},
			}

			// Second owner: nginx in a different namespace (production)
			deployment2 = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx",
					Namespace: "production",
					Labels:    map[string]string{"app": "nginx"},
					UID:       "deployment-uid-2",
				},
			}
			replicaSet2 = &appsv1.ReplicaSet{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-rs-0001",
					Namespace: "production",
					UID:       "replicaset-uid-2",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "nginx",
							UID:        "deployment-uid-2",
						},
					},
				},
			}
			pod2 = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-pod-2",
					Namespace: "production",
					UID:       "pod-uid-2",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "ReplicaSet",
							Name:       "nginx-rs-0001",
							UID:        "replicaset-uid-2",
						},
					},
				},
			}

			// Third owner: different name in nginx namespace
			deployment3 = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "apache",
					Namespace: "nginx",
					Labels:    map[string]string{"app": "nginx"},
					UID:       "deployment-uid-3",
				},
			}
			replicaSet3 = &appsv1.ReplicaSet{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "apache-rs-0001",
					Namespace: "nginx",
					UID:       "replicaset-uid-3",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "apache",
							UID:        "deployment-uid-3",
						},
					},
				},
			}
			pod3 = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "apache-pod-1",
					Namespace: "nginx",
					UID:       "pod-uid-3",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "ReplicaSet",
							Name:       "apache-rs-0001",
							UID:        "replicaset-uid-3",
						},
					},
				},
			}

			s := runtime.NewScheme()
			err := scheme.AddToScheme(s)
			Expect(err).NotTo(HaveOccurred())

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(deployment1, replicaSet1, pod1).
				WithObjects(deployment2, replicaSet2, pod2).
				WithObjects(deployment3, replicaSet3, pod3).
				Build()

			configuration.CreateControllerConfigOrDie(
				filepath.Join(tmpDir, "config.yaml"),
				configuration.WithClient(fakeClient),
				configuration.WithLog(_log),
			)

			localPodWebhook = &webhook.PodMutator{
				Client:  fakeClient,
				Decoder: admission.NewDecoder(s),
			}
		})

		It("should generate different cookie secrets for owners with different namespaces", func() {
			patchedPod1 := patchPodWithWebhook(pod1, localPodWebhook)
			cookieSecret1 := extractCookieSecret(patchedPod1)

			patchedPod2 := patchPodWithNamespaceAndWebhook(pod2, "production", localPodWebhook)
			cookieSecret2 := extractCookieSecret(patchedPod2)

			Expect(cookieSecret1).NotTo(BeEmpty(), "Cookie secret 1 should not be empty")
			Expect(cookieSecret2).NotTo(BeEmpty(), "Cookie secret 2 should not be empty")
			Expect(cookieSecret1).NotTo(Equal(cookieSecret2),
				"Different namespaces should produce different cookie secrets")

			_log.Info("Different namespace secrets verified",
				"namespace1", "nginx", "secret1", cookieSecret1,
				"namespace2", "production", "secret2", cookieSecret2)
		})

		It("should generate different cookie secrets for owners with different names", func() {
			patchedPod1 := patchPodWithWebhook(pod1, localPodWebhook)
			cookieSecret1 := extractCookieSecret(patchedPod1)

			patchedPod3 := patchPodWithWebhook(pod3, localPodWebhook)
			cookieSecret3 := extractCookieSecret(patchedPod3)

			Expect(cookieSecret1).NotTo(BeEmpty(), "Cookie secret 1 should not be empty")
			Expect(cookieSecret3).NotTo(BeEmpty(), "Cookie secret 3 should not be empty")
			Expect(cookieSecret1).NotTo(Equal(cookieSecret3),
				"Different owner names should produce different cookie secrets")

			_log.Info("Different name secrets verified",
				"name1", "nginx", "secret1", cookieSecret1,
				"name3", "apache", "secret3", cookieSecret3)
		})

		It("should ensure all three different owners have unique secrets", func() {
			patchedPod1 := patchPodWithWebhook(pod1, localPodWebhook)
			cookieSecret1 := extractCookieSecret(patchedPod1)

			patchedPod2 := patchPodWithNamespaceAndWebhook(pod2, "production", localPodWebhook)
			cookieSecret2 := extractCookieSecret(patchedPod2)

			patchedPod3 := patchPodWithWebhook(pod3, localPodWebhook)
			cookieSecret3 := extractCookieSecret(patchedPod3)

			// All three should be different
			Expect(cookieSecret1).NotTo(Equal(cookieSecret2))
			Expect(cookieSecret1).NotTo(Equal(cookieSecret3))
			Expect(cookieSecret2).NotTo(Equal(cookieSecret3))

			_log.Info("All unique secrets verified",
				"secret1", cookieSecret1,
				"secret2", cookieSecret2,
				"secret3", cookieSecret3)
		})
	})

	Context("when HTTPRoute is enabled, host annotation uses HTTPRoute hostPrefix", func() {
		var (
			deployment      *appsv1.Deployment
			replicaSet      *appsv1.ReplicaSet
			pod             *corev1.Pod
			localPodWebhook *webhook.PodMutator
			savedGlobal     configuration.Global
			savedTargets    []configuration.Target
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx",
					Namespace: "nginx",
					Labels:    map[string]string{"app": "nginx"},
					UID:       "deployment-uid-httproute",
				},
			}
			replicaSet = &appsv1.ReplicaSet{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-rs-0001",
					Namespace: "nginx",
					UID:       "replicaset-uid-httproute",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "nginx",
							UID:        "deployment-uid-httproute",
						},
					},
				},
			}
			pod = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-pod-1",
					Namespace: "nginx",
					UID:       "pod-uid-httproute",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "ReplicaSet",
							Name:       "nginx-rs-0001",
							UID:        "replicaset-uid-httproute",
						},
					},
				},
			}

			s := runtime.NewScheme()
			err := scheme.AddToScheme(s)
			Expect(err).NotTo(HaveOccurred())

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(deployment, replicaSet, pod).
				Build()

			cfg := configuration.GetOIDCAppsControllerConfig()
			savedGlobal = cfg.Global
			savedTargets = cfg.Targets

			cfg.Global = configuration.Global{
				DomainName: "example.org",
				Gateway:    &configuration.GatewayGlobalConf{HTTPRoutes: &configuration.HTTPRoutesConf{Enabled: true}},
				Oauth2Proxy: &configuration.Oauth2ProxyConfig{
					ClientID:      "client-id",
					ClientSecret:  "client-secret",
					OidcIssuerURL: "https://oidc-provider.org",
				},
			}
			cfg.Targets = []configuration.Target{
				{
					Name: "nginx",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "nginx"},
					},
					HTTPRoute: &configuration.HTTPRouteConf{
						Create:     true,
						HostPrefix: "my-httproute-prefix",
					},
				},
			}
			cfg.SetClient(fakeClient)

			localPodWebhook = &webhook.PodMutator{
				Client:  fakeClient,
				Decoder: admission.NewDecoder(s),
			}
		})

		It("should set the host annotation with the HTTPRoute hostPrefix", func() {
			patchedPod := patchPodWithWebhook(pod, localPodWebhook)

			hostAnnotation := patchedPod.GetAnnotations()[constants.AnnotationHostKey]
			Expect(hostAnnotation).NotTo(BeEmpty(), "Host annotation should be set")
			Expect(hostAnnotation).To(HavePrefix("my-httproute-prefix-"),
				"Host annotation should use the HTTPRoute hostPrefix")
			Expect(hostAnnotation).To(HaveSuffix(".example.org"),
				"Host annotation should include the global domain")
		})

		AfterEach(func() {
			cfg := configuration.GetOIDCAppsControllerConfig()
			cfg.Global = savedGlobal
			cfg.Targets = savedTargets
		})
	})

	Context("when extraArgs are configured with valid and invalid entries", func() {
		var (
			deployment      *appsv1.Deployment
			replicaSet      *appsv1.ReplicaSet
			pod             *corev1.Pod
			localPodWebhook *webhook.PodMutator
			savedGlobal     configuration.Global
			savedTargets    []configuration.Target
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx",
					Namespace: "nginx",
					Labels:    map[string]string{"app": "nginx"},
					UID:       "deployment-uid-extraargs",
				},
			}
			replicaSet = &appsv1.ReplicaSet{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-rs-0001",
					Namespace: "nginx",
					UID:       "replicaset-uid-extraargs",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "nginx",
							UID:        "deployment-uid-extraargs",
						},
					},
				},
			}
			pod = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-pod-1",
					Namespace: "nginx",
					UID:       "pod-uid-extraargs",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "ReplicaSet",
							Name:       "nginx-rs-0001",
							UID:        "replicaset-uid-extraargs",
						},
					},
				},
			}

			s := runtime.NewScheme()
			err := scheme.AddToScheme(s)
			Expect(err).NotTo(HaveOccurred())

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(deployment, replicaSet, pod).
				Build()

			cfg := configuration.GetOIDCAppsControllerConfig()
			savedGlobal = cfg.Global
			savedTargets = cfg.Targets

			cfg.Global = configuration.Global{
				DomainName: "example.org",
				Oauth2Proxy: &configuration.Oauth2ProxyConfig{
					ClientID:      "client-id",
					ClientSecret:  "client-secret",
					OidcIssuerURL: "https://oidc-provider.org",
					ExtraArgs: []string{
						"--set-xauthrequest=true",
						"invalid-no-dashes",
						"--valid-flag",
						"not a flag at all",
					},
				},
			}
			cfg.Targets = []configuration.Target{
				{
					Name: "nginx",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "nginx"},
					},
				},
			}
			cfg.SetClient(fakeClient)

			localPodWebhook = &webhook.PodMutator{
				Client:  fakeClient,
				Decoder: admission.NewDecoder(s),
			}
		})

		It("should include valid extraArgs and skip invalid ones", func() {
			patchedPod := patchPodWithWebhook(pod, localPodWebhook)

			var oauth2Args []string
			for _, container := range patchedPod.Spec.Containers {
				if container.Name == constants.ContainerNameOauth2Proxy {
					oauth2Args = container.Args

					break
				}
			}

			Expect(oauth2Args).To(ContainElement("--set-xauthrequest=true"))
			Expect(oauth2Args).To(ContainElement("--valid-flag"))
			Expect(oauth2Args).NotTo(ContainElement("invalid-no-dashes"))
			Expect(oauth2Args).NotTo(ContainElement("not a flag at all"))
		})

		AfterEach(func() {
			cfg := configuration.GetOIDCAppsControllerConfig()
			cfg.Global = savedGlobal
			cfg.Targets = savedTargets
		})
	})

	Context("VPA in-place update scenario integration test", func() {
		var (
			deployment      *appsv1.Deployment
			replicaSet      *appsv1.ReplicaSet
			initialPod      *corev1.Pod
			updatedPod      *corev1.Pod
			localPodWebhook *webhook.PodMutator
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx",
					Namespace: "nginx",
					Labels:    map[string]string{"app": "nginx"},
					UID:       "deployment-uid-vpa",
				},
			}
			replicaSet = &appsv1.ReplicaSet{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-rs-0001",
					Namespace: "nginx",
					UID:       "replicaset-uid-vpa",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "nginx",
							UID:        "deployment-uid-vpa",
						},
					},
				},
			}

			// Initial pod before VPA update
			initialPod = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-pod-vpa",
					Namespace: "nginx",
					UID:       "pod-uid-vpa",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "ReplicaSet",
							Name:       "nginx-rs-0001",
							UID:        "replicaset-uid-vpa",
						},
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.19",
						},
					},
				},
			}

			// Updated pod after VPA in-place update (resources changed)
			updatedPod = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-pod-vpa",
					Namespace: "nginx",
					UID:       "pod-uid-vpa",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "ReplicaSet",
							Name:       "nginx-rs-0001",
							UID:        "replicaset-uid-vpa",
						},
					},
					// VPA adds annotations during in-place update
					Annotations: map[string]string{
						"vpa.autoscaling.k8s.io/last-update": "2026-02-17T10:00:00Z",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.19",
							// VPA may update resource requests
						},
					},
				},
			}

			s := runtime.NewScheme()
			err := scheme.AddToScheme(s)
			Expect(err).NotTo(HaveOccurred())

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(deployment, replicaSet, initialPod).
				Build()

			configuration.CreateControllerConfigOrDie(
				filepath.Join(tmpDir, "config.yaml"),
				configuration.WithClient(fakeClient),
				configuration.WithLog(_log),
			)

			localPodWebhook = &webhook.PodMutator{
				Client:  fakeClient,
				Decoder: admission.NewDecoder(s),
			}
		})

		It("should maintain the same cookie secret when VPA performs in-place updates", func() {
			// Simulate initial pod creation
			patchedInitialPod := patchPodWithWebhook(initialPod, localPodWebhook)
			initialCookieSecret := extractCookieSecret(patchedInitialPod)

			// Simulate VPA in-place update (pod spec changes, but owner remains the same)
			patchedUpdatedPod := patchPodWithWebhook(updatedPod, localPodWebhook)
			updatedCookieSecret := extractCookieSecret(patchedUpdatedPod)

			Expect(initialCookieSecret).NotTo(BeEmpty(), "Initial cookie secret should not be empty")
			Expect(updatedCookieSecret).NotTo(BeEmpty(), "Updated cookie secret should not be empty")
			Expect(initialCookieSecret).To(Equal(updatedCookieSecret),
				"Cookie secret should remain stable across VPA in-place updates")

			_log.Info("VPA in-place update stability verified",
				"initialSecret", initialCookieSecret,
				"updatedSecret", updatedCookieSecret)
		})

		It("should maintain cookie secret stability across multiple VPA updates", func() {
			const vpaUpdates = 3

			var cookieSecrets []string

			for i := range vpaUpdates {
				// Simulate pod with VPA update annotations
				podWithVPAUpdate := initialPod.DeepCopy()
				podWithVPAUpdate.Annotations = map[string]string{
					"vpa.autoscaling.k8s.io/update-count": string(rune('0' + i)),
				}

				patchedPod := patchPodWithWebhook(podWithVPAUpdate, localPodWebhook)
				cookieSecret := extractCookieSecret(patchedPod)
				cookieSecrets = append(cookieSecrets, cookieSecret)
			}

			// All cookie secrets should be identical
			for i := 1; i < vpaUpdates; i++ {
				Expect(cookieSecrets[i]).To(Equal(cookieSecrets[0]),
					"Cookie secret should remain stable across all VPA updates")
			}

			_log.Info("Multiple VPA updates stability verified",
				"updates", vpaUpdates,
				"consistentSecret", cookieSecrets[0])
		})

		It("should not cause forbidden pod spec changes due to cookie secret during VPA update", func() {
			// This test ensures that the cookie secret doesn't change between create and update operations
			// which would cause a forbidden pod spec change error in Kubernetes

			// Simulate CREATE operation
			patchedOnCreate := patchPodWithOperationAndWebhook(initialPod, adminssionv1.Create, localPodWebhook)
			cookieSecretOnCreate := extractCookieSecret(patchedOnCreate)

			// Simulate UPDATE operation (VPA in-place update scenario)
			patchedOnUpdate := patchPodWithOperationAndWebhook(updatedPod, adminssionv1.Update, localPodWebhook)
			cookieSecretOnUpdate := extractCookieSecret(patchedOnUpdate)

			Expect(cookieSecretOnCreate).To(Equal(cookieSecretOnUpdate),
				"Cookie secret should be identical between CREATE and UPDATE operations to avoid forbidden pod spec changes")

			_log.Info("CREATE vs UPDATE consistency verified",
				"createSecret", cookieSecretOnCreate,
				"updateSecret", cookieSecretOnUpdate)
		})
	})

	Context("when kubeRbacProxy kubeConfigStr and kubeSecretRef are not configured", func() {
		var (
			deployment      *appsv1.Deployment
			replicaSet      *appsv1.ReplicaSet
			pod             *corev1.Pod
			localPodWebhook *webhook.PodMutator
			savedGlobal     configuration.Global
			savedTargets    []configuration.Target
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx",
					Namespace: "nginx",
					Labels:    map[string]string{"app": "nginx"},
					UID:       "deployment-uid-sa-token",
				},
			}
			replicaSet = &appsv1.ReplicaSet{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-rs-0001",
					Namespace: "nginx",
					UID:       "replicaset-uid-sa-token",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "nginx",
							UID:        "deployment-uid-sa-token",
						},
					},
				},
			}
			pod = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-pod-1",
					Namespace: "nginx",
					UID:       "pod-uid-sa-token",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "ReplicaSet",
							Name:       "nginx-rs-0001",
							UID:        "replicaset-uid-sa-token",
						},
					},
				},
				Spec: corev1.PodSpec{
					// Simulate the kube-api-access projected volume injected by kubelet.
					Volumes: []corev1.Volume{
						{
							Name: "kube-api-access-abcde",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									Sources: []corev1.VolumeProjection{
										{ServiceAccountToken: &corev1.ServiceAccountTokenProjection{Path: "token"}},
										{ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "kube-root-ca.crt"}}},
									},
								},
							},
						},
					},
				},
			}

			s := runtime.NewScheme()
			err := scheme.AddToScheme(s)
			Expect(err).NotTo(HaveOccurred())

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(deployment, replicaSet, pod).
				Build()

			cfg := configuration.GetOIDCAppsControllerConfig()
			savedGlobal = cfg.Global
			savedTargets = cfg.Targets

			// Configure without any kubeRbacProxy kubeconfig — kube-rbac-proxy should
			// use the pod's service account token for SubjectAccessReviews instead.
			cfg.Global = configuration.Global{
				DomainName: "example.org",
				Oauth2Proxy: &configuration.Oauth2ProxyConfig{
					ClientID:      "client-id",
					ClientSecret:  "client-secret",
					OidcIssuerURL: "https://oidc-provider.org",
				},
				// KubeRbacProxy intentionally nil: no kubeConfigStr, no kubeSecretRef
			}
			cfg.Targets = []configuration.Target{
				{
					Name: "nginx",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "nginx"},
					},
				},
			}
			cfg.SetClient(fakeClient)

			localPodWebhook = &webhook.PodMutator{
				Client:  fakeClient,
				Decoder: admission.NewDecoder(s),
			}
		})

		It("should not add --kubeconfig arg to kube-rbac-proxy when no kubeconfig is configured", func() {
			patchedPod := patchPodWithWebhook(pod, localPodWebhook)

			var rbacArgs []string

			for _, container := range patchedPod.Spec.Containers {
				if container.Name == constants.ContainerNameKubeRbacProxy {
					rbacArgs = container.Args

					break
				}
			}

			Expect(rbacArgs).NotTo(BeEmpty(), "kube-rbac-proxy container should be present")

			for _, arg := range rbacArgs {
				Expect(arg).NotTo(HavePrefix("--kubeconfig="),
					"kube-rbac-proxy should not receive --kubeconfig when no kubeconfig is configured")
			}
		})

		It("should mount the pod service account token volume into kube-rbac-proxy when no kubeconfig is configured", func() {
			patchedPod := patchPodWithWebhook(pod, localPodWebhook)

			var rbacMounts []corev1.VolumeMount

			for _, container := range patchedPod.Spec.Containers {
				if container.Name == constants.ContainerNameKubeRbacProxy {
					rbacMounts = container.VolumeMounts

					break
				}
			}

			Expect(rbacMounts).NotTo(BeEmpty(), "kube-rbac-proxy should have volume mounts")

			var hasSAMount bool

			for _, mount := range rbacMounts {
				if mount.MountPath == "/var/run/secrets/kubernetes.io/serviceaccount" {
					hasSAMount = true

					Expect(mount.ReadOnly).To(BeTrue(), "SA token mount should be read-only")

					break
				}
			}

			Expect(hasSAMount).To(BeTrue(),
				"kube-rbac-proxy should mount the pod SA token at /var/run/secrets/kubernetes.io/serviceaccount "+
					"so it can use in-cluster auth for SubjectAccessReviews without a Dex OIDC token")
		})

		It("should not include a kubeconfig source in the kube-rbac-proxy projected volume when no kubeconfig is configured", func() {
			patchedPod := patchPodWithWebhook(pod, localPodWebhook)

			for _, vol := range patchedPod.Spec.Volumes {
				if vol.Name != constants.KubeRbacProxyVolumeName {
					continue
				}

				if vol.Projected == nil {
					break
				}

				for _, src := range vol.Projected.Sources {
					if src.Secret != nil {
						for _, item := range src.Secret.Items {
							Expect(item.Key).NotTo(Equal("kubeconfig"),
								"kube-rbac-proxy projected volume should not include a kubeconfig secret source "+
									"when kubeConfigStr and kubeSecretRef are both empty")
						}
					}
				}
			}
		})

		AfterEach(func() {
			cfg := configuration.GetOIDCAppsControllerConfig()
			cfg.Global = savedGlobal
			cfg.Targets = savedTargets
		})
	})
})

func extractCookieSecret(pod *corev1.Pod) string {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.ContainerNameOauth2Proxy {
			for _, arg := range container.Args {
				if after, ok := strings.CutPrefix(arg, "--cookie-secret="); ok {
					return after
				}
			}
		}
	}

	return ""
}

// Helper function to patch pod with a specific webhook
func patchPodWithWebhook(pod *corev1.Pod, wh *webhook.PodMutator) *corev1.Pod {
	return patchPodWithNamespaceOperationAndWebhook(pod, pod.Namespace, adminssionv1.Create, wh)
}

// Helper function to patch pod with a specific namespace and webhook
func patchPodWithNamespaceAndWebhook(pod *corev1.Pod, namespace string, wh *webhook.PodMutator) *corev1.Pod {
	return patchPodWithNamespaceOperationAndWebhook(pod, namespace, adminssionv1.Create, wh)
}

// Helper function to patch pod with a specific operation and webhook
func patchPodWithOperationAndWebhook(pod *corev1.Pod, operation adminssionv1.Operation, wh *webhook.PodMutator) *corev1.Pod {
	return patchPodWithNamespaceOperationAndWebhook(pod, pod.Namespace, operation, wh)
}

// Helper function to patch pod with namespace, operation, and webhook
func patchPodWithNamespaceOperationAndWebhook(pod *corev1.Pod, namespace string, operation adminssionv1.Operation, wh *webhook.PodMutator) *corev1.Pod {
	raw, err := json.Marshal(pod)
	Expect(err).NotTo(HaveOccurred())

	req := admission.Request{
		AdmissionRequest: adminssionv1.AdmissionRequest{
			UID:       "uid-request",
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Namespace: namespace,
			Operation: operation,
			Object: runtime.RawExtension{
				Raw: raw,
			},
		},
	}

	resp := wh.Handle(context.Background(), req)
	_log.Info("webhook response", "response", resp.String())
	Expect(resp.Allowed).To(BeTrue())

	// If no patches, return original pod
	if resp.Patches == nil {
		return pod
	}

	patchBytes, err := json.Marshal(resp.Patches)
	Expect(err).NotTo(HaveOccurred())
	decodedPatch, err := jsonpatch.DecodePatch(patchBytes)
	Expect(err).NotTo(HaveOccurred())

	// Apply the patch
	patchedPodBytes, err := decodedPatch.Apply(raw)
	Expect(err).NotTo(HaveOccurred())

	var patchedPod = &corev1.Pod{}

	err = json.Unmarshal(patchedPodBytes, &patchedPod)
	Expect(err).NotTo(HaveOccurred())

	return patchedPod
}
