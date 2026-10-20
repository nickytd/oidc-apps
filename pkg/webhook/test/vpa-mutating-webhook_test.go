// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	autoscalerv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
	"github.com/nickytd/oidc-apps/pkg/webhook"
)

var targetVPA, nonTargetVpa *autoscalerv1.VerticalPodAutoscaler
var vpaWebhook *webhook.VPAMutator

var _ = BeforeEach(func() {
	s := runtime.NewScheme()
	err := scheme.AddToScheme(s)
	Expect(err).NotTo(HaveOccurred())
	err = autoscalerv1.AddToScheme(s)
	Expect(err).NotTo(HaveOccurred())

	initTargetDeployment()
	initNonTargetDeployment()
	initTargetVPA()
	initNonTargetVPA()

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(targetDeployment, targetReplicaSet, targetPod).
		WithObjects(targetVPA).
		WithObjects(nonTargetDeployment, nonTargetReplicaSet, nonTargetPod).
		WithObjects(nonTargetVpa).
		Build()

	extCfg := configuration.CreateControllerConfigOrDie(
		filepath.Join(tmpDir, "config.yaml"),
		configuration.WithClient(fakeClient),
		configuration.WithLog(_log),
	)

	vpaWebhook = &webhook.VPAMutator{
		Client: fakeClient,
		Config: extCfg,
	}
})

func initTargetVPA() {
	targetVPA = &autoscalerv1.VerticalPodAutoscaler{
		Name:      "nginx-vpa",
		Namespace: "nginx",
		Labels:    map[string]string{"app": "nginx"},
		Spec: autoscalerv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "nginx",
				APIVersion: "apps/v1",
			},
			UpdatePolicy: &autoscalerv1.PodUpdatePolicy{
				UpdateMode: new(autoscalerv1.UpdateModeRecreate),
			},
			ResourcePolicy: &autoscalerv1.PodResourcePolicy{
				ContainerPolicies: []autoscalerv1.ContainerResourcePolicy{
					{
						ContainerName: "nginx",
						MinAllowed: corev1.ResourceList{
							"cpu":    resource.MustParse("100m"),
							"memory": resource.MustParse("100Mi"),
						},
					},
				},
			},
		},
	}
}

func initNonTargetVPA() {
	nonTargetVpa = &autoscalerv1.VerticalPodAutoscaler{
		Name:      "test-vpa",
		Namespace: "default",
		Spec: autoscalerv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "test",
				APIVersion: "apps/v1",
			},
			UpdatePolicy: &autoscalerv1.PodUpdatePolicy{
				UpdateMode: new(autoscalerv1.UpdateModeRecreate),
			},
			ResourcePolicy: &autoscalerv1.PodResourcePolicy{
				ContainerPolicies: []autoscalerv1.ContainerResourcePolicy{
					{
						ContainerName: "test",
						MinAllowed: corev1.ResourceList{
							"cpu":    resource.MustParse("100m"),
							"memory": resource.MustParse("100Mi"),
						},
					},
				},
			},
		},
	}
}

var _ = Describe("Oidc Apps MutatingAdmission Framework Test", func() {
	Context("when the reconciled target has vpa", func() {
		It("It shall contain mode=Off for auth & autz proxies", func() {
			patchedVpa := patchVpa(targetVPA)
			_log.Info("patched pod", "patched vpa", patchedVpa)
			Expect(patchedVpa.Spec.ResourcePolicy.ContainerPolicies).To(HaveLen(3))
			Expect(patchedVpa.Spec.ResourcePolicy.ContainerPolicies).To(ContainElement(autoscalerv1.ContainerResourcePolicy{
				ContainerName: constants.ContainerNameOauth2Proxy,
				Mode:          new(autoscalerv1.ContainerScalingModeOff),
			}))
			Expect(patchedVpa.Spec.ResourcePolicy.ContainerPolicies).To(ContainElement(autoscalerv1.ContainerResourcePolicy{
				ContainerName: constants.ContainerNameKubeRbacProxy,
				Mode:          new(autoscalerv1.ContainerScalingModeOff),
			}))
		})
	})

	Context("when the reconciled target has a vpa without a resource policy", func() {
		It("It shall initialize the policy with mode=Off for the proxies", func() {
			vpa := targetVPA.DeepCopy()
			vpa.Spec.ResourcePolicy = nil

			err := vpaWebhook.Default(context.Background(), vpa)
			Expect(err).NotTo(HaveOccurred())

			Expect(vpa.Spec.ResourcePolicy).NotTo(BeNil())
			Expect(vpa.Spec.ResourcePolicy.ContainerPolicies).To(ContainElement(autoscalerv1.ContainerResourcePolicy{
				ContainerName: constants.ContainerNameOauth2Proxy,
				Mode:          new(autoscalerv1.ContainerScalingModeOff),
			}))
			Expect(vpa.Spec.ResourcePolicy.ContainerPolicies).To(ContainElement(autoscalerv1.ContainerResourcePolicy{
				ContainerName: constants.ContainerNameKubeRbacProxy,
				Mode:          new(autoscalerv1.ContainerScalingModeOff),
			}))
		})
	})

	Context("when a reconciled target does not belong to a target", func() {
		It("there shall be no auth & authz policies in the vpa spec", func() {
			nonTarget := nonTargetVpa.DeepCopy()
			policies := len(nonTarget.Spec.ResourcePolicy.ContainerPolicies)

			err := vpaWebhook.Default(context.Background(), nonTarget)
			Expect(err).NotTo(HaveOccurred())

			// A non-target vpa must be left untouched.
			Expect(nonTarget.Spec.ResourcePolicy.ContainerPolicies).To(HaveLen(policies))
		}) // It
	})
})

func patchVpa(vpa *autoscalerv1.VerticalPodAutoscaler) *autoscalerv1.VerticalPodAutoscaler {
	patchedVpa := vpa.DeepCopy()

	err := vpaWebhook.Default(context.Background(), patchedVpa)
	Expect(err).NotTo(HaveOccurred())

	return patchedVpa
}
