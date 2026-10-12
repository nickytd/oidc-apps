// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"
	"slices"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
)

// Register the webhook with the server
var _ admission.Defaulter[*corev1.Pod] = &PodMutator{}

// PodMutator is a handler modifying the resource definitions of the pod targets
type PodMutator struct {
	Client          client.Client
	ImagePullSecret string
	Config          *configuration.OIDCAppsControllerConfig
}

// Default provides the admission.Defaulter implementation for the PodMutator,
// injecting the proxy sidecars into target pods in place.
func (p *PodMutator) Default(ctx context.Context, pod *corev1.Pod) error {
	_log := log.FromContext(ctx)

	// Simply return if the pod is being deleted
	if !pod.GetDeletionTimestamp().IsZero() {
		return nil
	}

	// Simply return if the pod is not part of the described targets
	target, owner := isTarget(ctx, p.Client, p.Config, pod)
	if !target {
		return nil
	}

	_log.Info("handling pod admission request")

	rc := p.Config.Resolve(ctx, owner)

	clientID := rc.GetClientID()
	issuerURL := rc.GetOidcIssuerURL()
	upstream := rc.GetUpstreamTarget()
	upstreamURL := buildUpstreamURL(upstream, pod.Spec)
	suffix := fetchTargetSuffix(owner)

	// Add the OIDC annotation to the deployment template
	addAnnotations(pod, p.Config, rc)

	// Add required annotations to pod spec template
	addPodAnnotations(pod,
		map[string]string{
			constants.AnnotationHostKey: rc.GetHost(),
		},
	)

	// Add required labels to pod spec template
	addPodLabels(pod,
		map[string]string{
			constants.LabelKey: "pod",
		},
	)

	// Add the oauth2-proxy volume
	addProjectedSecretSourceVolume(
		constants.Oauth2VolumeName,
		constants.SecretNameOauth2Proxy+"-"+suffix,
		&pod.Spec,
	)

	if shallAddOidcCaSecretName(rc) {
		addProjectedSecretSourceVolume(
			constants.Oauth2VolumeName,
			fetchOidcCASecretName(suffix, rc),
			&pod.Spec,
		)
	}

	// Add the resource-attribute secret volume for the kube-rbac-proxy
	addProjectedSecretSourceVolume(
		constants.KubeRbacProxyVolumeName,
		constants.SecretNameResourceAttributes+"-"+suffix,
		&pod.Spec,
	)

	// Add an optional kubeconfig secret for the kube-rbac-proxy
	if shallAddKubeConfigSecretName(rc) {
		addProjectedSecretSourceVolume(
			constants.KubeRbacProxyVolumeName,
			fetchKubconfigSecretName(suffix, rc),
			&pod.Spec,
		)
	}

	// Add an optional oidc ca secret for the kube-rbac-proxy
	if shallAddOidcCaSecretName(rc) {
		addProjectedSecretSourceVolume(
			constants.KubeRbacProxyVolumeName,
			fetchOidcCASecretName(suffix, rc),
			&pod.Spec,
		)
	}

	// Add the OAUTH2 proxy sidecar to the pod template
	addProxyContainer(constants.ContainerNameOauth2Proxy, &pod.Spec, getOIDCProxyContainer(_log, &pod.Spec, owner, rc))

	// Add the kube-rbac-proxy sidecar to the pod template
	addProxyContainer(constants.ContainerNameKubeRbacProxy, &pod.Spec, getKubeRbacProxyContainer(clientID, issuerURL, upstreamURL, pod, rc))

	// Add image pull secret if the proxy container images are served from private registry
	if len(p.ImagePullSecret) > 0 {
		addImagePullSecret(p.ImagePullSecret, &pod.Spec)
	}

	podIndex, present := pod.GetObjectMeta().GetLabels()["statefulset.kubernetes.io/pod-name"]

	// When "present", the target pod is part of a statefulset.
	// In this case, the oidc-apps-controller creates a new HTTPRoute for each pod and the oauth2-proxy redirect
	// URL needs to reflect the target pod index. When the redirect URL is constructed the pod index is appended to the
	// host name.
	if present {
		hostPrefix := rc.GetHost()
		if rc.GetRedirectURL() != "" {
			hostPrefix = rc.GetRedirectURL()
			// It does container https:// prefix and /oauth2/callback suffix
			hostPrefix = strings.TrimPrefix(hostPrefix, "https://")
			hostPrefix = strings.TrimSuffix(hostPrefix, "/oauth2/callback")
		}

		host, domain, found := strings.Cut(hostPrefix, ".")
		if found {
			l := strings.Split(podIndex, "-")
			host = fmt.Sprintf("%s-%s.%s", host, l[len(l)-1], domain)
		}

		for idx, container := range pod.Spec.Containers {
			if container.Name != "oauth2-proxy" {
				continue
			}
			// Remove the argument if present
			for i, arg := range container.Args {
				if strings.HasPrefix(arg, "--redirect-url") {
					pod.Spec.Containers[idx].Args = slices.Delete(pod.Spec.Containers[idx].Args, i, i+1)
				}
			}
			// Add the correct argument
			pod.Spec.Containers[idx].Args = append(pod.Spec.Containers[idx].Args,
				fmt.Sprintf("--redirect-url=https://%s/oauth2/callback", host),
			)

			break
		}
	}

	return nil
}

func isTarget(ctx context.Context, c client.Client, cfg *configuration.OIDCAppsControllerConfig, pod *corev1.Pod) (bool, client.Object) {
	// Identify the workload
	owners := pod.GetOwnerReferences()
	if len(owners) == 0 {
		return false, nil
	}

	for _, o := range owners {
		if o.Kind == "StatefulSet" {
			statefulset := &appsv1.StatefulSet{}

			err := c.Get(ctx, client.ObjectKey{Name: o.Name, Namespace: pod.GetNamespace()}, statefulset)
			if err != nil {
				log.FromContext(ctx).Error(err, "unable to get statefulset for object", "object", pod)

				return false, nil
			}

			return cfg.Match(ctx, statefulset), statefulset
		}

		if o.Kind == "ReplicaSet" {
			replicaset := &appsv1.ReplicaSet{}
			if err := c.Get(ctx, client.ObjectKey{Name: o.Name, Namespace: pod.GetNamespace()}, replicaset); err != nil {
				log.FromContext(ctx).Error(err, "unable to get replicaset for object", "object", pod)

				return false, nil
			}

			ownerRefs := replicaset.GetOwnerReferences()
			if len(ownerRefs) == 0 {
				log.FromContext(ctx).Info("replicaset has no owner references", "replicaset", replicaset.GetName())

				return false, nil
			}

			if ownerRefs[0].Kind != "Deployment" {
				log.FromContext(ctx).Info("replicaset owner is not a deployment", "replicaset", replicaset.GetName(), "ownerKind", ownerRefs[0].Kind)

				return false, nil
			}

			deployment := &appsv1.Deployment{}

			err := c.Get(ctx, client.ObjectKey{Name: ownerRefs[0].Name, Namespace: pod.GetNamespace()}, deployment)
			if err != nil {
				log.FromContext(ctx).Error(err, "unable to get deployment for object", "object", pod)

				return false, nil
			}

			return cfg.Match(ctx, deployment), deployment
		}
	}

	return false, nil
}
