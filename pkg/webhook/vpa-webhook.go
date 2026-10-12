// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalerv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
)

// Register the webhook with the server
var _ admission.Defaulter[*autoscalerv1.VerticalPodAutoscaler] = &VPAMutator{}

// VPAMutator is a handler modifying the resource definitions of the vpa targets
type VPAMutator struct {
	Client client.Client
	Config *configuration.OIDCAppsControllerConfig
}

// Default provides the admission.Defaulter implementation for the VPAMutator,
// pinning the proxy sidecar container policies to Mode=Off in place.
func (v *VPAMutator) Default(ctx context.Context, vpa *autoscalerv1.VerticalPodAutoscaler) error {
	_log := log.FromContext(ctx)

	// Simply return if the vpa is being deleted
	if !vpa.GetDeletionTimestamp().IsZero() {
		return nil
	}

	// Check if VPA target ref
	if vpa.Spec.TargetRef == nil {
		return nil
	}

	if !v.Config.Match(ctx, vpa) {
		if !isTargetRefMatched(ctx, v.Client, v.Config, vpa.GetNamespace(), vpa.Spec.TargetRef) {
			return nil
		}
	}

	_log.Info("handling vpa admission request")

	// The resource policy is optional on the submitted VPA; initialize it so the
	// proxy container policies below can always be set.
	if vpa.Spec.ResourcePolicy == nil {
		vpa.Spec.ResourcePolicy = &autoscalerv1.PodResourcePolicy{}
	}

	policies := make([]autoscalerv1.ContainerResourcePolicy, 0)

	for i, policy := range vpa.Spec.ResourcePolicy.ContainerPolicies {
		if policy.ContainerName != constants.ContainerNameOauth2Proxy &&
			policy.ContainerName != constants.ContainerNameKubeRbacProxy {
			policies = append(policies, vpa.Spec.ResourcePolicy.ContainerPolicies[i])
		}
	}

	for _, name := range []string{constants.ContainerNameOauth2Proxy, constants.ContainerNameKubeRbacProxy} {
		policies = append(policies, autoscalerv1.ContainerResourcePolicy{
			ContainerName: name,
			Mode:          new(autoscalerv1.ContainerScalingModeOff),
		})
	}

	vpa.Spec.ResourcePolicy.ContainerPolicies = policies

	return nil
}

func isTargetRefMatched(ctx context.Context, c client.Client, cfg *configuration.OIDCAppsControllerConfig, namespace string, ref *autoscalingv1.CrossVersionObjectReference) bool {
	switch ref.Kind {
	case "Deployment":
		deployment := &appsv1.Deployment{}
		if err := c.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, deployment); err != nil {
			log.FromContext(ctx).V(9).Info("unable to get deployment", "name", ref.Name)

			return false
		}

		return cfg.Match(ctx, deployment)

	case "StatefulSet":
		statefulset := &appsv1.StatefulSet{}
		if err := c.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, statefulset); err != nil {
			log.FromContext(ctx).V(9).Info("unable to get statefulset", "name", ref.Name)

			return false
		}

		return cfg.Match(ctx, statefulset)
	default:
	}

	return false
}
