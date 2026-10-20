// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	_ "embed"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
	"github.com/nickytd/oidc-apps/pkg/randutils"
	"github.com/nickytd/oidc-apps/pkg/webhook"
)

var targetDeployment, nonTargetDeployment *appsv1.Deployment
var targetReplicaSet, nonTargetReplicaSet *appsv1.ReplicaSet
var targetPod, targetPodWithServiceAccount, podWithLessResources, podWithMoreResources, nonTargetPod *corev1.Pod
var podWebhook *webhook.PodMutator

var _ = BeforeEach(func() {
	initNonTargetDeployment()
	initTargetDeployment()

	s := runtime.NewScheme()
	err := scheme.AddToScheme(s)
	Expect(err).NotTo(HaveOccurred())

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(targetDeployment, targetReplicaSet, targetPod).
		WithObjects(podWithLessResources, podWithMoreResources, targetPodWithServiceAccount).
		WithObjects(nonTargetDeployment, nonTargetReplicaSet, nonTargetPod).
		Build()

	extCfg := configuration.CreateControllerConfigOrDie(
		filepath.Join(tmpDir, "config.yaml"),
		configuration.WithClient(fakeClient),
		configuration.WithLog(_log),
	)

	podWebhook = &webhook.PodMutator{
		Client: fakeClient,
		Config: extCfg,
	}
})

func initTargetDeployment() {
	targetDeployment = &appsv1.Deployment{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "nginx",
		Namespace:  "nginx",
		Labels:     map[string]string{"app": "nginx"},
		UID:        "target-deployment",
	}
	targetReplicaSet = &appsv1.ReplicaSet{
		APIVersion: "apps/v1",
		Kind:       "ReplicaSet",
		Name:       "nginx-rs-0001",
		Namespace:  "nginx",
		UID:        "target-replicaset",
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "nginx",
				UID:        "target-deployment",
			},
		},
	}
	targetPod = &corev1.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       "nginx",
		Namespace:  "nginx",
		UID:        "uid-pod",
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "nginx-rs-0001",
				UID:        "target-replicaset",
			},
		},
	}

	targetPodWithServiceAccount = &corev1.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       "nginx-with-sa",
		Namespace:  "nginx",
		UID:        "uid-pod",
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "nginx-rs-0001",
				UID:        "target-replicaset",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "nginx",
		},
	}

	podWithLessResources = &corev1.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       "nginx-with-less-resources",
		Namespace:  "nginx",
		UID:        "uid-pod",
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "nginx-rs-0001",
				UID:        "target-replicaset",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "nginx",
				},
				{
					Name: constants.ContainerNameOauth2Proxy,
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							"cpu":    resource.MustParse("1m"),
							"memory": resource.MustParse("10Mi"),
						},
					},
				},
			},
		},
	}

	podWithMoreResources = &corev1.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       "nginx-with-more-resources",
		Namespace:  "nginx",
		UID:        "uid-pod",
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "nginx-rs-0001",
				UID:        "target-replicaset",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "nginx",
				},
				{
					Name: constants.ContainerNameOauth2Proxy,
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							"cpu":    resource.MustParse("500m"),
							"memory": resource.MustParse("300Mi"),
						},
					},
				},
			},
		},
	}
}

func initNonTargetDeployment() {
	nonTargetDeployment = &appsv1.Deployment{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "test",
		Namespace:  "default",
		UID:        "non-target-deployment",
	}
	nonTargetReplicaSet = &appsv1.ReplicaSet{
		APIVersion: "apps/v1",
		Kind:       "ReplicaSet",
		Name:       "test-rs-0001",
		Namespace:  "default",
		UID:        "non-target-replicaset",
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "test",
				UID:        "non-target-deployment",
			},
		},
	}

	nonTargetPod = &corev1.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       "test",
		Namespace:  "default",
		UID:        "uid-pod",
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "tesst-rs-0001",
				UID:        "non-target-replicaset",
			},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{
			{
				Name:  "test",
				Image: "test",
			},
		}},
	}
}

var _ = Describe("Oidc Apps MutatingAdmission Framework Test", func() {
	Context("when a pod belongs to a target", func() {
		It("there shall be auth & authz proxies in the patch pod spec", func() {
			patchedPod := patchPod(targetPod)
			_log.Info("patched pod", "patched pod", patchedPod)
			By("verifying containers in patched pod", func() {
				expectedContainerImages := []string{"kube-rbac-proxy", "oauth2-proxy"}

				Expect(len(patchedPod.Spec.Containers)).To(Equal(2))

				for _, c := range patchedPod.Spec.Containers {
					image, _, ok := strings.Cut(c.Image, ":")

					Expect(ok).To(BeTrue())

					n := strings.SplitAfter(image, "/")
					Expect(n[len(n)-1]).To(BeElementOf(expectedContainerImages))
				}
			}) // By
		}) // It
		It("there shall be a secret volumes in the target pod", func() {
			patchedPod := patchPod(targetPod)
			_log.Info("patched pod volumes", "patched pod", patchedPod.Spec.Volumes)
			Expect(patchedPod.Spec.Volumes).To(ContainElement(
				corev1.Volume{
					Name: constants.Oauth2VolumeName,
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									Name:     "oauth2-proxy-" + randutils.GenerateSha256(targetDeployment.Name+"-"+targetDeployment.Namespace),
									Optional: new(false),
								},
							},
							{
								Secret: &corev1.SecretProjection{
									Name: "oidc-ca-" + randutils.GenerateSha256(targetDeployment.
										Name+"-"+targetDeployment.Namespace),
									Optional: new(false),
								},
							},
						},
					},
				},
			))
			Expect(patchedPod.Spec.Volumes).To(ContainElement(
				corev1.Volume{
					Name: constants.KubeRbacProxyVolumeName,
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									Name:     "resource-attributes-" + randutils.GenerateSha256(targetDeployment.Name+"-"+targetDeployment.Namespace),
									Optional: new(false),
								},
							},
							{
								Secret: &corev1.SecretProjection{
									Name: "oidc-ca-" + randutils.GenerateSha256(targetDeployment.
										Name+"-"+targetDeployment.Namespace),
									Optional: new(false),
								},
							},
						},
					},
				},
			))
		})
		It("there shall be a volume mounts for auth & authz proxies", func() {
			patchedPod := patchPod(targetPod)
			for _, c := range patchedPod.Spec.Containers {
				_log.Info("patched pod container", "container", c)

				switch c.Name {
				case constants.ContainerNameOauth2Proxy:
					Expect(c.VolumeMounts).To(ContainElement(
						corev1.VolumeMount{
							Name:      constants.Oauth2VolumeName,
							ReadOnly:  true,
							MountPath: "/etc/oauth2-proxy",
						}))
				case constants.ContainerNameKubeRbacProxy:
					Expect(c.VolumeMounts).To(ContainElement(
						corev1.VolumeMount{
							Name:      constants.KubeRbacProxyVolumeName,
							ReadOnly:  true,
							MountPath: "/etc/kube-rbac-proxy",
						}))
				default:
				}
			}
		})
		When("the target configuration has a oidcCABundle", func() {
			It("there shall be a projected secret volume in the pod spec containing oidc-ca secret", func() {
				patchedPod := patchPod(targetPod)
				_log.Info("patched pod volumes", "patched pod", patchedPod.Spec.Volumes)
				Expect(patchedPod.Spec.Volumes).To(ContainElement(
					corev1.Volume{
						Name: constants.KubeRbacProxyVolumeName,
						Projected: &corev1.ProjectedVolumeSource{
							Sources: []corev1.VolumeProjection{
								{
									Secret: &corev1.SecretProjection{
										Name:     "resource-attributes-" + randutils.GenerateSha256(targetDeployment.Name+"-"+targetDeployment.Namespace),
										Optional: new(false),
									},
								},
								{
									Secret: &corev1.SecretProjection{
										Name: "oidc-ca-" + randutils.GenerateSha256(targetDeployment.
											Name+"-"+targetDeployment.Namespace),
										Optional: new(false),
									},
								},
							},
						},
					},
				))
			}) // It
		}) // When
		When("there is a container resource defined in the incoming request which are less than default", func() {
			It("shall modify the container resources", func() {
				pp := patchPod(podWithLessResources)
				_log.Info("patched pod", "patched pod", pp)

				for _, c := range pp.Spec.Containers {
					if c.Name == constants.ContainerNameOauth2Proxy {
						Expect(c.Resources).To(Equal(corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu":    resource.MustParse("5m"),
								"memory": resource.MustParse("32Mi"),
							},
							Claims: nil,
						}))
					}
				}
			})
		}) // When there is a container resource defined in the incoming request
		When("there is a container resource defined in the incoming request which are bigger than default", func() {
			It("shall not modify the container resources", func() {
				pp := patchPod(podWithMoreResources)

				_log.Info("patched pod", "patched pod", pp)

				for _, c := range pp.Spec.Containers {
					if c.Name == constants.ContainerNameOauth2Proxy {
						Expect(c.Resources).To(Equal(podWithMoreResources.Spec.Containers[1].Resources))
					}
				}
			})
		})
		When("there isn't any container resource defined in the incoming request", func() {
			It("shall set the default container resources", func() {
				pp := patchPod(podWithLessResources)

				_log.Info("patched pod", "patched pod", pp)

				for _, c := range pp.Spec.Containers {
					if c.Name == constants.ContainerNameKubeRbacProxy {
						expected := corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								"cpu":    resource.MustParse("5m"),
								"memory": resource.MustParse("32Mi"),
							}}
						Expect(c.Resources).To(Equal(expected))
					}
				}
			})
		}) // When there isn't any container resource defined in the incoming request
	}) // Context
	Context("when a pod does not belong to a target", func() {
		It("there shall be no auth & authz proxies in the pod templates spec", func() {
			nonTarget := nonTargetPod.DeepCopy()
			containers := len(nonTarget.Spec.Containers)

			err := podWebhook.Default(context.Background(), nonTarget)
			Expect(err).NotTo(HaveOccurred())

			// A non-target pod must be left untouched.
			Expect(nonTarget.Spec.Containers).To(HaveLen(containers))
		}) // It
	}) // Context when a pod does not belong to a target
}) // Describe

func patchPod(pod *corev1.Pod) *corev1.Pod {
	patchedPod := pod.DeepCopy()

	err := podWebhook.Default(context.Background(), patchedPod)
	Expect(err).NotTo(HaveOccurred())

	return patchedPod
}
