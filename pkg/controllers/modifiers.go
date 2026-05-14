// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	autoscalerv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
)

func reconcileOwnedResource(ctx context.Context, c client.Client, owner client.Object, obj client.Object, mutate func() error) error {
	result, err := controllerutil.CreateOrUpdate(ctx, c, obj, func() error {
		if err := mutate(); err != nil {
			return err
		}

		return controllerutil.SetOwnerReference(owner, obj, c.Scheme())
	})
	if err != nil {
		return err
	}

	if result != controllerutil.OperationResultNone {
		log.FromContext(ctx).Info("reconciled resource",
			"kind", fmt.Sprintf("%T", obj),
			"name", obj.GetName(),
			"operation", result,
		)
	}

	return nil
}

func fetchOidcAppsServices(ctx context.Context, c client.Client, object client.Object) (*corev1.ServiceList, error) {
	oidcService := &corev1.ServiceList{}

	if err := c.List(ctx, oidcService,
		client.InNamespace(object.GetNamespace()),
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(map[string]string{
				constants.LabelKey: constants.LabelValue,
			}),
		},
	); err != nil {
		return oidcService, client.IgnoreNotFound(err)
	}

	ownedServices := make([]corev1.Service, 0, len(oidcService.Items))

	for _, service := range oidcService.Items {
		if isAnOwnedResource(object, &service) {
			ownedServices = append(ownedServices, service)
		}
	}

	return &corev1.ServiceList{Items: ownedServices}, nil
}

func fetchOidcAppsIngress(ctx context.Context, c client.Client, object client.Object) (*networkingv1.IngressList,
	error) {
	oidcIngress := &networkingv1.IngressList{}

	if err := c.List(ctx, oidcIngress,
		client.InNamespace(object.GetNamespace()),
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(map[string]string{
				constants.LabelKey: constants.LabelValue,
			}),
		},
	); err != nil {
		return oidcIngress, client.IgnoreNotFound(err)
	}

	ownedIngresses := make([]networkingv1.Ingress, 0, len(oidcIngress.Items))

	for _, ingress := range oidcIngress.Items {
		if isAnOwnedResource(object, &ingress) {
			ownedIngresses = append(ownedIngresses, ingress)
		}
	}

	return &networkingv1.IngressList{Items: ownedIngresses}, nil
}

func fetchOidcAppsHTTPRoutes(ctx context.Context, c client.Client, object client.Object) (*gatewayv1.HTTPRouteList,
	error) {
	if !configuration.GetOIDCAppsControllerConfig().IsHTTPRouteEnabled() {
		return &gatewayv1.HTTPRouteList{}, nil
	}

	oidcHTTPRoutes := &gatewayv1.HTTPRouteList{}

	if err := c.List(ctx, oidcHTTPRoutes,
		client.InNamespace(object.GetNamespace()),
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(map[string]string{
				constants.LabelKey: constants.LabelValue,
			}),
		},
	); err != nil {
		return oidcHTTPRoutes, client.IgnoreNotFound(err)
	}

	ownedHTTPRoutes := make([]gatewayv1.HTTPRoute, 0, len(oidcHTTPRoutes.Items))

	for _, httpRoute := range oidcHTTPRoutes.Items {
		if isAnOwnedResource(object, &httpRoute) {
			ownedHTTPRoutes = append(ownedHTTPRoutes, httpRoute)
		}
	}

	return &gatewayv1.HTTPRouteList{Items: ownedHTTPRoutes}, nil
}

func fetchOidcAppsSecrets(ctx context.Context, c client.Client, object client.Object) (*corev1.SecretList,
	error) {
	oidcSecrets := &corev1.SecretList{}

	if err := c.List(ctx, oidcSecrets,
		client.InNamespace(object.GetNamespace()),
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(map[string]string{
				constants.LabelKey:       constants.LabelValue,
				constants.SecretLabelKey: constants.Oauth2LabelValue,
			}),
		},
	); err != nil {
		return oidcSecrets, client.IgnoreNotFound(err)
	}

	ownedSecrets := make([]corev1.Secret, 0, len(oidcSecrets.Items))

	for _, secret := range oidcSecrets.Items {
		if isAnOwnedResource(object, &secret) {
			ownedSecrets = append(ownedSecrets, secret)
		}
	}

	return &corev1.SecretList{Items: ownedSecrets}, nil
}

func fetchResourceAttributesNamespace(_ context.Context, _ client.Client, object client.Object) string {
	return object.GetNamespace()
}

// reconcileDeploymentDependencies is the function responsible for managing authentication & authorization dependencies.
// It reconciles the needed secrets, ingresses and services.
func reconcileDeploymentDependencies(ctx context.Context, c client.Client, object *appsv1.Deployment) error {
	if !object.GetDeletionTimestamp().IsZero() {
		return nil
	}

	// OAuth2 secret
	oauth2Secret := oauth2SecretObject(object)
	if err := reconcileOwnedResource(ctx, c, object, oauth2Secret, func() error {
		return mutateOauth2Secret(oauth2Secret, object)
	}); err != nil {
		return fmt.Errorf("failed to reconcile oauth2 secret: %w", err)
	}

	// OAuth2 service
	selectors := configuration.GetOIDCAppsControllerConfig().GetTargetLabelSelector(object)

	oauth2Svc := oauth2ServiceObject(object)
	if err := reconcileOwnedResource(ctx, c, object, oauth2Svc, func() error {
		return mutateOauth2Service(oauth2Svc, selectors.MatchLabels)
	}); err != nil {
		return fmt.Errorf("failed to reconcile oauth2 service: %w", err)
	}

	// Resource attributes secret
	ns := fetchResourceAttributesNamespace(ctx, c, object)

	rbacSecret := resourceAttributesSecretObject(object)
	if err := reconcileOwnedResource(ctx, c, object, rbacSecret, func() error {
		return mutateResourceAttributesSecret(rbacSecret, object, ns)
	}); err != nil {
		return fmt.Errorf("failed to reconcile resource attributes secret: %w", err)
	}

	// Optional kubeconfig secret
	if needsKubeconfigSecret(object) {
		kubeSecret := kubeconfigSecretObject(object)
		if err := reconcileOwnedResource(ctx, c, object, kubeSecret, func() error {
			return mutateKubeconfigSecret(kubeSecret, object)
		}); err != nil {
			return fmt.Errorf("failed to reconcile kubeconfig secret: %w", err)
		}
	}

	// Optional OIDC CA bundle secret
	if needsOidcCaBundleSecret(object) {
		caSecret := oidcCaBundleSecretObject(object)
		if err := reconcileOwnedResource(ctx, c, object, caSecret, func() error {
			return mutateOidcCaBundleSecret(caSecret, object)
		}); err != nil {
			return fmt.Errorf("failed to reconcile oidc ca bundle secret: %w", err)
		}
	}

	if err := reconcileIngressForDeployment(ctx, c, object); err != nil {
		return err
	}

	if err := reconcileHTTPRouteForDeployment(ctx, c, object); err != nil {
		return err
	}

	if c.Scheme().IsGroupRegistered("autoscaling.k8s.io") {
		return patchVpa(ctx, c, object)
	}

	return nil
}

func reconcileIngressForDeployment(ctx context.Context, c client.Client, object client.Object) error {
	if !configuration.GetOIDCAppsControllerConfig().ShallCreateIngress(object) {
		return nil
	}

	ingress := ingressForDeploymentObject(object)
	if err := reconcileOwnedResource(ctx, c, object, ingress, func() error {
		return mutateIngressForDeployment(ingress, object)
	}); err != nil {
		return fmt.Errorf("failed to reconcile oauth2 ingress: %w", err)
	}

	return nil
}

func reconcileIngressForStatefulSetPod(ctx context.Context, c client.Client, pod *corev1.Pod,
	object client.Object) error {
	if !configuration.GetOIDCAppsControllerConfig().ShallCreateIngress(object) {
		return nil
	}

	ingress := ingressForStatefulSetPodObject(pod, object)
	if err := reconcileOwnedResource(ctx, c, pod, ingress, func() error {
		return mutateIngressForStatefulSetPod(ingress, pod, object)
	}); err != nil {
		return fmt.Errorf("failed to reconcile oauth2 ingress: %w", err)
	}

	return nil
}

func reconcileHTTPRouteForDeployment(ctx context.Context, c client.Client, object client.Object) error {
	if !configuration.GetOIDCAppsControllerConfig().ShallCreateHTTPRoute(object) {
		return nil
	}

	httpRoute := httpRouteForDeploymentObject(object)
	if err := reconcileOwnedResource(ctx, c, object, httpRoute, func() error {
		return mutateHTTPRouteForDeployment(httpRoute, object)
	}); err != nil {
		return fmt.Errorf("failed to reconcile oauth2 httproute: %w", err)
	}

	return nil
}

func reconcileHTTPRouteForStatefulSetPod(ctx context.Context, c client.Client, pod *corev1.Pod,
	object client.Object) error {
	if !configuration.GetOIDCAppsControllerConfig().ShallCreateHTTPRoute(object) {
		return nil
	}

	httpRoute := httpRouteForStatefulSetPodObject(pod, object)
	if err := reconcileOwnedResource(ctx, c, pod, httpRoute, func() error {
		return mutateHTTPRouteForStatefulSetPod(httpRoute, pod, object)
	}); err != nil {
		return fmt.Errorf("failed to reconcile oauth2 httproute: %w", err)
	}

	return nil
}

func reconcileStatefulSetDependencies(ctx context.Context, c client.Client, object *appsv1.StatefulSet) error {
	if !object.GetDeletionTimestamp().IsZero() {
		return nil
	}

	// OAuth2 secret
	oauth2Secret := oauth2SecretObject(object)
	if err := reconcileOwnedResource(ctx, c, object, oauth2Secret, func() error {
		return mutateOauth2Secret(oauth2Secret, object)
	}); err != nil {
		return fmt.Errorf("failed to reconcile oauth2 secret: %w", err)
	}

	// For each pod in the statefulset
	podList := &corev1.PodList{}

	labelSelector := client.MatchingLabels(object.Spec.Selector.MatchLabels)
	if err := c.List(ctx, podList, labelSelector, client.InNamespace(object.GetNamespace())); err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range podList.Items {
		log.FromContext(ctx).V(9).Info("Reconciling pod", "pod", pod.GetName(), "annotations", pod.GetAnnotations())

		_, found := pod.GetAnnotations()[constants.AnnotationHostKey]
		if !found {
			continue
		}

		// OAuth2 service per pod
		selectors := client.MatchingLabels{}
		if configuration.GetOIDCAppsControllerConfig().GetTargetLabelSelector(&pod) != nil {
			selectors = configuration.GetOIDCAppsControllerConfig().GetTargetLabelSelector(&pod).MatchLabels
		}

		if statefulSetPodNameLabel, ok := pod.GetLabels()["statefulset.kubernetes.io/pod-name"]; ok {
			selectors = map[string]string{"statefulset.kubernetes.io/pod-name": statefulSetPodNameLabel}
		}

		oauth2Svc := oauth2ServiceObject(&pod)
		if err := reconcileOwnedResource(ctx, c, &pod, oauth2Svc, func() error {
			return mutateOauth2Service(oauth2Svc, selectors)
		}); err != nil {
			return fmt.Errorf("failed to reconcile oauth2 service: %w", err)
		}

		if err := reconcileIngressForStatefulSetPod(ctx, c, &pod, object); err != nil {
			return err
		}

		if err := reconcileHTTPRouteForStatefulSetPod(ctx, c, &pod, object); err != nil {
			return err
		}
	}

	// Resource attributes secret
	ns := fetchResourceAttributesNamespace(ctx, c, object)

	rbacSecret := resourceAttributesSecretObject(object)
	if err := reconcileOwnedResource(ctx, c, object, rbacSecret, func() error {
		return mutateResourceAttributesSecret(rbacSecret, object, ns)
	}); err != nil {
		return fmt.Errorf("failed to reconcile resource attributes secret: %w", err)
	}

	// Optional kubeconfig secret
	if needsKubeconfigSecret(object) {
		kubeSecret := kubeconfigSecretObject(object)
		if err := reconcileOwnedResource(ctx, c, object, kubeSecret, func() error {
			return mutateKubeconfigSecret(kubeSecret, object)
		}); err != nil {
			return fmt.Errorf("failed to reconcile kubeconfig secret: %w", err)
		}
	}

	// Optional OIDC CA bundle secret
	if needsOidcCaBundleSecret(object) {
		caSecret := oidcCaBundleSecretObject(object)
		if err := reconcileOwnedResource(ctx, c, object, caSecret, func() error {
			return mutateOidcCaBundleSecret(caSecret, object)
		}); err != nil {
			return fmt.Errorf("failed to reconcile oidc ca bundle secret: %w", err)
		}
	}

	if c.Scheme().IsGroupRegistered("autoscaling.k8s.io") {
		return patchVpa(ctx, c, object)
	}

	return nil
}

func patchVpa(ctx context.Context, c client.Client, object client.Object) error {
	vpa := &autoscalerv1.VerticalPodAutoscalerList{}
	targetLabels := configuration.GetOIDCAppsControllerConfig().GetTargetLabelSelector(object)

	listOpts := []client.ListOption{
		client.MatchingLabels(targetLabels.MatchLabels),
		client.InNamespace(object.GetNamespace()),
	}
	if err := c.List(ctx, vpa, listOpts...); err != nil {
		return fmt.Errorf("failed to list vpas: %w", err)
	}

	for i, v := range vpa.Items {
		containerPolicies := v.Spec.ResourcePolicy.ContainerPolicies
		for _, policy := range containerPolicies {
			if policy.ContainerName == constants.ContainerNameOauth2Proxy || policy.ContainerName == constants.ContainerNameKubeRbacProxy {
				continue
			}

			if err := c.Patch(ctx, &vpa.Items[i], client.RawPatch(types.MergePatchType, []byte(`{}`))); err != nil {
				return fmt.Errorf("failed to patch vpa: %w", err)
			}

			log.FromContext(ctx).Info("trigger patch", "vpa", v.GetName())
		}
	}

	return nil
}

func addOptionalIndex(idx string) string {
	if idx == "-" {
		return ""
	}

	idxStr, ok := strings.CutSuffix(idx, "-")
	if !ok {
		return ""
	}

	i, err := strconv.ParseInt(idxStr, 0, 32)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%d-", i)
}

func hasOidcAppsPods(ctx context.Context, c client.Client, object client.Object) bool {
	_log := log.FromContext(ctx)

	podList := &corev1.PodList{}
	if err := c.List(ctx, podList, client.InNamespace(object.GetNamespace())); err != nil {
		_log.Error(err, "unable to list pods", "namespace", object.GetNamespace())

		return false
	}

	for _, pod := range podList.Items {
		if !isOidcAppPod(pod) {
			continue
		}

		for _, ref := range pod.GetOwnerReferences() {
			switch ref.Kind {
			case "StatefulSet":
				if ref.UID == object.GetUID() {
					return true
				}
			case "ReplicaSet":
				rs := &appsv1.ReplicaSet{}
				if err := c.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: object.GetNamespace()}, rs); client.IgnoreNotFound(err) != nil {
					log.FromContext(ctx).Error(err, "cannot get replicaset", "name", ref.Name)

					return false
				}

				for _, d := range rs.OwnerReferences {
					if d.Kind == "Deployment" && d.UID == object.GetUID() {
						return true
					}
				}
			default:
			}
		}
	}

	return false
}

func isOidcAppPod(pod corev1.Pod) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name == constants.ContainerNameOauth2Proxy || c.Name == constants.ContainerNameKubeRbacProxy {
			return true
		}
	}

	return false
}
