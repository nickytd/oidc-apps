// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
	"github.com/nickytd/oidc-apps/pkg/webhook"
)

var _ = Describe("Cookie Secret File Reference Tests", func() {
	Context("when injecting the oauth2-proxy cookie secret", func() {
		var (
			deployment1      *appsv1.Deployment
			replicaSet1      *appsv1.ReplicaSet
			pod1             *corev1.Pod
			localPodWebhook1 *webhook.PodMutator
		)

		BeforeEach(func() {
			deployment1 = &appsv1.Deployment{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "nginx",
				Namespace:  "nginx",
				Labels:     map[string]string{"app": "nginx"},
				UID:        "deployment-uid-1",
			}
			replicaSet1 = &appsv1.ReplicaSet{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "nginx-rs-0001",
				Namespace:  "nginx",
				UID:        "replicaset-uid-1",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "nginx",
						UID:        "deployment-uid-1",
					},
				},
			}
			pod1 = &corev1.Pod{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "nginx-pod-1",
				Namespace:  "nginx",
				UID:        "pod-uid-1",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "ReplicaSet",
						Name:       "nginx-rs-0001",
						UID:        "replicaset-uid-1",
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

			extCfg := configuration.CreateControllerConfigOrDie(
				filepath.Join(tmpDir, "config.yaml"),
				configuration.WithClient(fakeClient),
				configuration.WithLog(_log),
			)

			localPodWebhook1 = &webhook.PodMutator{
				Client: fakeClient,
				Config: extCfg,
			}
		})

		It("should reference the cookie secret via a file, not an inline value", func() {
			patchedPod := patchPodWithWebhook(pod1, localPodWebhook1)

			args := oauth2ProxyArgs(patchedPod)
			Expect(args).To(ContainElement(
				"--cookie-secret-file=/etc/oauth2-proxy/"+constants.Oauth2CookieSecretKey),
				"oauth2-proxy must read the cookie secret from the mounted secret file")
		})

		It("should not pass the cookie secret as an inline CLI argument", func() {
			patchedPod := patchPodWithWebhook(pod1, localPodWebhook1)

			for _, arg := range oauth2ProxyArgs(patchedPod) {
				Expect(arg).NotTo(HavePrefix("--cookie-secret="),
					"the cookie secret value must never appear in the pod spec / process args")
			}
		})

		It("should emit a stable file arg across repeated admissions (VPA in-place safety)", func() {
			patchedPod1 := patchPodWithWebhook(pod1, localPodWebhook1)
			patchedPod2 := patchPodWithWebhook(pod1, localPodWebhook1)

			Expect(oauth2ProxyCookieSecretFileArg(patchedPod1)).NotTo(BeEmpty())
			Expect(oauth2ProxyCookieSecretFileArg(patchedPod1)).
				To(Equal(oauth2ProxyCookieSecretFileArg(patchedPod2)),
					"the cookie-secret-file arg must be identical across admissions")
		})
	})

	Context("when HTTPRoute is enabled, host annotation uses HTTPRoute hostPrefix", func() {
		var (
			deployment      *appsv1.Deployment
			replicaSet      *appsv1.ReplicaSet
			pod             *corev1.Pod
			localPodWebhook *webhook.PodMutator
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "nginx",
				Namespace:  "nginx",
				Labels:     map[string]string{"app": "nginx"},
				UID:        "deployment-uid-httproute",
			}
			replicaSet = &appsv1.ReplicaSet{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "nginx-rs-0001",
				Namespace:  "nginx",
				UID:        "replicaset-uid-httproute",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "nginx",
						UID:        "deployment-uid-httproute",
					},
				},
			}
			pod = &corev1.Pod{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "nginx-pod-1",
				Namespace:  "nginx",
				UID:        "pod-uid-httproute",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "ReplicaSet",
						Name:       "nginx-rs-0001",
						UID:        "replicaset-uid-httproute",
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

			extCfg := &configuration.OIDCAppsControllerConfig{
				Global: configuration.Global{
					DomainName: "example.org",
					Oauth2Proxy: &configuration.Oauth2ProxyConfig{
						ClientID:      "client-id",
						ClientSecret:  "client-secret",
						OidcIssuerURL: "https://oidc-provider.org",
					},
				},
				Targets: []configuration.Target{
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
				},
			}
			extCfg.SetClient(fakeClient)

			localPodWebhook = &webhook.PodMutator{
				Client: fakeClient,
				Config: extCfg,
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
	})

	Context("when extraArgs are configured with valid and invalid entries", func() {
		var (
			deployment      *appsv1.Deployment
			replicaSet      *appsv1.ReplicaSet
			pod             *corev1.Pod
			localPodWebhook *webhook.PodMutator
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "nginx",
				Namespace:  "nginx",
				Labels:     map[string]string{"app": "nginx"},
				UID:        "deployment-uid-extraargs",
			}
			replicaSet = &appsv1.ReplicaSet{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "nginx-rs-0001",
				Namespace:  "nginx",
				UID:        "replicaset-uid-extraargs",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "nginx",
						UID:        "deployment-uid-extraargs",
					},
				},
			}
			pod = &corev1.Pod{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "nginx-pod-1",
				Namespace:  "nginx",
				UID:        "pod-uid-extraargs",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "ReplicaSet",
						Name:       "nginx-rs-0001",
						UID:        "replicaset-uid-extraargs",
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

			extCfg := &configuration.OIDCAppsControllerConfig{
				Global: configuration.Global{
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
				},
				Targets: []configuration.Target{
					{
						Name: "nginx",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "nginx"},
						},
					},
				},
			}
			extCfg.SetClient(fakeClient)

			localPodWebhook = &webhook.PodMutator{
				Client: fakeClient,
				Config: extCfg,
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
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "nginx",
				Namespace:  "nginx",
				Labels:     map[string]string{"app": "nginx"},
				UID:        "deployment-uid-vpa",
			}
			replicaSet = &appsv1.ReplicaSet{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "nginx-rs-0001",
				Namespace:  "nginx",
				UID:        "replicaset-uid-vpa",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "nginx",
						UID:        "deployment-uid-vpa",
					},
				},
			}

			// Initial pod before VPA update
			initialPod = &corev1.Pod{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "nginx-pod-vpa",
				Namespace:  "nginx",
				UID:        "pod-uid-vpa",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "ReplicaSet",
						Name:       "nginx-rs-0001",
						UID:        "replicaset-uid-vpa",
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
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "nginx-pod-vpa",
				Namespace:  "nginx",
				UID:        "pod-uid-vpa",
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

			extCfg := configuration.CreateControllerConfigOrDie(
				filepath.Join(tmpDir, "config.yaml"),
				configuration.WithClient(fakeClient),
				configuration.WithLog(_log),
			)

			localPodWebhook = &webhook.PodMutator{
				Client: fakeClient,
				Config: extCfg,
			}
		})

		It("should keep the cookie-secret-file arg stable across VPA in-place updates", func() {
			// Simulate initial pod creation
			patchedInitialPod := patchPodWithWebhook(initialPod, localPodWebhook)
			initialArg := oauth2ProxyCookieSecretFileArg(patchedInitialPod)

			// Simulate VPA in-place update (pod spec changes, but owner remains the same)
			patchedUpdatedPod := patchPodWithWebhook(updatedPod, localPodWebhook)
			updatedArg := oauth2ProxyCookieSecretFileArg(patchedUpdatedPod)

			Expect(initialArg).NotTo(BeEmpty(), "Initial cookie-secret-file arg should not be empty")
			Expect(updatedArg).NotTo(BeEmpty(), "Updated cookie-secret-file arg should not be empty")
			Expect(initialArg).To(Equal(updatedArg),
				"cookie-secret-file arg should remain stable across VPA in-place updates")

			_log.Info("VPA in-place update stability verified",
				"initialArg", initialArg,
				"updatedArg", updatedArg)
		})

		It("should keep the cookie-secret-file arg stable across multiple VPA updates", func() {
			const vpaUpdates = 3

			var args []string

			for i := range vpaUpdates {
				// Simulate pod with VPA update annotations
				podWithVPAUpdate := initialPod.DeepCopy()
				podWithVPAUpdate.Annotations = map[string]string{
					"vpa.autoscaling.k8s.io/update-count": string(rune('0' + i)),
				}

				patchedPod := patchPodWithWebhook(podWithVPAUpdate, localPodWebhook)
				args = append(args, oauth2ProxyCookieSecretFileArg(patchedPod))
			}

			// All args should be identical
			for i := 1; i < vpaUpdates; i++ {
				Expect(args[i]).To(Equal(args[0]),
					"cookie-secret-file arg should remain stable across all VPA updates")
			}

			_log.Info("Multiple VPA updates stability verified",
				"updates", vpaUpdates,
				"consistentArg", args[0])
		})

		It("should not inject a per-admission cookie secret value that would cause forbidden pod spec changes", func() {
			// The webhook must reference the secret by file, never inject the value inline,
			// so CREATE and UPDATE admissions produce identical pod specs.

			// Simulate CREATE operation
			patchedOnCreate := patchPodWithWebhook(initialPod, localPodWebhook)
			argOnCreate := oauth2ProxyCookieSecretFileArg(patchedOnCreate)

			// Simulate UPDATE operation (VPA in-place update scenario)
			patchedOnUpdate := patchPodWithWebhook(updatedPod, localPodWebhook)
			argOnUpdate := oauth2ProxyCookieSecretFileArg(patchedOnUpdate)

			Expect(argOnCreate).To(Equal(argOnUpdate),
				"cookie-secret-file arg should be identical between CREATE and UPDATE operations")

			for _, arg := range oauth2ProxyArgs(patchedOnUpdate) {
				Expect(arg).NotTo(HavePrefix("--cookie-secret="),
					"the webhook must never inject the cookie secret value inline")
			}

			_log.Info("CREATE vs UPDATE consistency verified",
				"createArg", argOnCreate,
				"updateArg", argOnUpdate)
		})
	})

	Context("when kubeRbacProxy kubeConfigStr and kubeSecretRef are not configured", func() {
		var (
			deployment      *appsv1.Deployment
			replicaSet      *appsv1.ReplicaSet
			pod             *corev1.Pod
			localPodWebhook *webhook.PodMutator
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "nginx",
				Namespace:  "nginx",
				Labels:     map[string]string{"app": "nginx"},
				UID:        "deployment-uid-sa-token",
			}
			replicaSet = &appsv1.ReplicaSet{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "nginx-rs-0001",
				Namespace:  "nginx",
				UID:        "replicaset-uid-sa-token",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "nginx",
						UID:        "deployment-uid-sa-token",
					},
				},
			}
			pod = &corev1.Pod{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "nginx-pod-1",
				Namespace:  "nginx",
				UID:        "pod-uid-sa-token",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "ReplicaSet",
						Name:       "nginx-rs-0001",
						UID:        "replicaset-uid-sa-token",
					},
				},
				Spec: corev1.PodSpec{
					// Simulate the kube-api-access projected volume injected by kubelet.
					Volumes: []corev1.Volume{
						{
							Name: "kube-api-access-abcde",
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{
									{ServiceAccountToken: &corev1.ServiceAccountTokenProjection{Path: "token"}},
									{ConfigMap: &corev1.ConfigMapProjection{Name: "kube-root-ca.crt"}},
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

			// Configure without any kubeRbacProxy kubeconfig — kube-rbac-proxy should
			// use the pod's service account token for SubjectAccessReviews instead.
			extCfg := &configuration.OIDCAppsControllerConfig{
				Global: configuration.Global{
					DomainName: "example.org",
					Oauth2Proxy: &configuration.Oauth2ProxyConfig{
						ClientID:      "client-id",
						ClientSecret:  "client-secret",
						OidcIssuerURL: "https://oidc-provider.org",
					},
					// KubeRbacProxy intentionally nil: no kubeConfigStr, no kubeSecretRef
				},
				Targets: []configuration.Target{
					{
						Name: "nginx",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "nginx"},
						},
					},
				},
			}
			extCfg.SetClient(fakeClient)

			localPodWebhook = &webhook.PodMutator{
				Client: fakeClient,
				Config: extCfg,
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
	})

	Context("when a clientSecretRef is configured", func() {
		var (
			deployment      *appsv1.Deployment
			replicaSet      *appsv1.ReplicaSet
			pod             *corev1.Pod
			localPodWebhook *webhook.PodMutator
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "nginx",
				Namespace:  "nginx",
				Labels:     map[string]string{"app": "nginx"},
				UID:        "deployment-uid-csr",
			}
			replicaSet = &appsv1.ReplicaSet{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "nginx-rs-0001",
				Namespace:  "nginx",
				UID:        "replicaset-uid-csr",
				OwnerReferences: []metav1.OwnerReference{
					{APIVersion: "apps/v1", Kind: "Deployment", Name: "nginx", UID: "deployment-uid-csr"},
				},
			}
			pod = &corev1.Pod{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "nginx-pod-1",
				Namespace:  "nginx",
				UID:        "pod-uid-csr",
				OwnerReferences: []metav1.OwnerReference{
					{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "nginx-rs-0001", UID: "replicaset-uid-csr"},
				},
			}

			s := runtime.NewScheme()
			Expect(scheme.AddToScheme(s)).To(Succeed())

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(deployment, replicaSet, pod).
				Build()

			extCfg := &configuration.OIDCAppsControllerConfig{
				Global: configuration.Global{
					DomainName: "example.org",
					Oauth2Proxy: &configuration.Oauth2ProxyConfig{
						ClientID:        "client-id",
						OidcIssuerURL:   "https://oidc-provider.org",
						ClientSecretRef: &corev1.SecretReference{Name: "my-client-secret"},
					},
				},
				Targets: []configuration.Target{
					{
						Name:          "nginx",
						LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
					},
				},
			}
			extCfg.SetClient(fakeClient)

			localPodWebhook = &webhook.PodMutator{Client: fakeClient, Config: extCfg}
		})

		It("mounts the referenced secret into the oauth2-proxy projected volume", func() {
			patchedPod := patchPodWithWebhook(pod, localPodWebhook)

			Expect(oauth2VolumeHasSecretSource(patchedPod, "my-client-secret")).To(BeTrue(),
				"the clientSecretRef secret must be a source of the oauth2-proxy projected volume")
		})
	})
})

// oauth2VolumeHasSecretSource reports whether the oauth2-proxy projected volume includes a secret
// source with the given name.
func oauth2VolumeHasSecretSource(pod *corev1.Pod, secretName string) bool {
	for _, vol := range pod.Spec.Volumes {
		if vol.Name != constants.Oauth2VolumeName || vol.Projected == nil {
			continue
		}

		for _, src := range vol.Projected.Sources {
			if src.Secret != nil && src.Secret.Name == secretName {
				return true
			}
		}
	}

	return false
}

// oauth2ProxyArgs returns the args of the oauth2-proxy container in the pod.
func oauth2ProxyArgs(pod *corev1.Pod) []string {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.ContainerNameOauth2Proxy {
			return container.Args
		}
	}

	return nil
}

// oauth2ProxyCookieSecretFileArg returns the value of the --cookie-secret-file arg
// on the oauth2-proxy container, or "" if not present.
func oauth2ProxyCookieSecretFileArg(pod *corev1.Pod) string {
	for _, arg := range oauth2ProxyArgs(pod) {
		if after, ok := strings.CutPrefix(arg, "--cookie-secret-file="); ok {
			return after
		}
	}

	return ""
}

// Helper function to patch pod with a specific webhook
func patchPodWithWebhook(pod *corev1.Pod, wh *webhook.PodMutator) *corev1.Pod {
	patchedPod := pod.DeepCopy()

	err := wh.Default(context.Background(), patchedPod)
	Expect(err).NotTo(HaveOccurred())

	return patchedPod
}
