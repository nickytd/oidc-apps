// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"fmt"
	"maps"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
	"github.com/nickytd/oidc-apps/pkg/randutils"
)

func httpRouteForDeploymentObject(object client.Object) *gatewayv1.HTTPRoute {
	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())

	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.HTTPRouteName + "-" + suffix,
			Namespace: object.GetNamespace(),
		},
	}
}

func mutateHTTPRouteForDeployment(httpRoute *gatewayv1.HTTPRoute, object client.Object) error {
	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())
	extConfig := configuration.GetOIDCAppsControllerConfig()
	host := extConfig.GetHTTPRouteHost(object)
	parentRefs := extConfig.GetHTTPRouteParentRefs(object)

	httpRoute.Labels = map[string]string{
		constants.LabelKey: constants.LabelValue,
	}
	httpRoute.Spec = gatewayv1.HTTPRouteSpec{
		CommonRouteSpec: gatewayv1.CommonRouteSpec{
			ParentRefs: convertParentRefs(parentRefs, object.GetNamespace()),
		},
		Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(host)},
		Rules: []gatewayv1.HTTPRouteRule{
			{
				Matches: []gatewayv1.HTTPRouteMatch{
					{
						Path: &gatewayv1.HTTPPathMatch{
							Type:  new(gatewayv1.PathMatchPathPrefix),
							Value: new("/"),
						},
					},
				},
				BackendRefs: []gatewayv1.HTTPBackendRef{
					{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: gatewayv1.ObjectName(constants.ServiceNameOauth2Service + "-" + suffix),
								Port: new(gatewayv1.PortNumber(8080)),
							},
						},
					},
				},
			},
		},
	}

	if annotations := extConfig.GetHTTPRouteAnnotations(object); len(annotations) > 0 {
		httpRoute.Annotations = annotations
	}

	applyHTTPRouteDefaultPathRedirect(httpRoute, object)

	extraLabels := extConfig.GetHTTPRouteLabels(object)
	maps.Copy(httpRoute.Labels, extraLabels)

	return nil
}

func httpRouteForStatefulSetPodObject(pod *corev1.Pod, object client.Object) *gatewayv1.HTTPRoute {
	suffix := randutils.GenerateSha256(pod.GetName() + "-" + pod.GetNamespace())
	index := fetchStrIndexIfPresent(pod)

	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.HTTPRouteName + "-" + addOptionalIndex(index+"-") + suffix,
			Namespace: object.GetNamespace(),
		},
	}
}

func mutateHTTPRouteForStatefulSetPod(httpRoute *gatewayv1.HTTPRoute, pod *corev1.Pod, object client.Object) error {
	suffix := randutils.GenerateSha256(pod.GetName() + "-" + pod.GetNamespace())
	extConfig := configuration.GetOIDCAppsControllerConfig()
	parentRefs := extConfig.GetHTTPRouteParentRefs(object)
	index := fetchStrIndexIfPresent(pod)

	hostPrefix, ok := pod.GetAnnotations()[constants.AnnotationHostKey]
	if !ok {
		return fmt.Errorf("host annotation not found in pod %s/%s", pod.GetNamespace(), pod.GetName())
	}

	host, domain, _ := strings.Cut(hostPrefix, ".")
	podHost := fmt.Sprintf("%s-%s.%s", host, index, domain)

	httpRoute.Labels = map[string]string{
		constants.LabelKey: constants.LabelValue,
	}
	httpRoute.Spec = gatewayv1.HTTPRouteSpec{
		CommonRouteSpec: gatewayv1.CommonRouteSpec{
			ParentRefs: convertParentRefs(parentRefs, object.GetNamespace()),
		},
		Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(podHost)},
		Rules: []gatewayv1.HTTPRouteRule{
			{
				Matches: []gatewayv1.HTTPRouteMatch{
					{
						Path: &gatewayv1.HTTPPathMatch{
							Type:  new(gatewayv1.PathMatchPathPrefix),
							Value: new("/"),
						},
					},
				},
				BackendRefs: []gatewayv1.HTTPBackendRef{
					{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: gatewayv1.ObjectName(constants.ServiceNameOauth2Service + "-" + addOptionalIndex(index+"-") + suffix),
								Port: new(gatewayv1.PortNumber(8080)),
							},
						},
					},
				},
			},
		},
	}

	if annotations := extConfig.GetHTTPRouteAnnotations(object); len(annotations) > 0 {
		httpRoute.Annotations = annotations
	}

	applyHTTPRouteDefaultPathRedirect(httpRoute, object)

	extraLabels := extConfig.GetHTTPRouteLabels(object)
	maps.Copy(httpRoute.Labels, extraLabels)

	return nil
}

func applyHTTPRouteDefaultPathRedirect(httpRoute *gatewayv1.HTTPRoute, object client.Object) {
	defaultPath := configuration.GetOIDCAppsControllerConfig().GetHTTPRouteDefaultPath(object)
	if defaultPath == "" {
		return
	}

	redirectRule := gatewayv1.HTTPRouteRule{
		Matches: []gatewayv1.HTTPRouteMatch{
			{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  new(gatewayv1.PathMatchExact),
					Value: new("/"),
				},
			},
		},
		Filters: []gatewayv1.HTTPRouteFilter{
			{
				Type: gatewayv1.HTTPRouteFilterRequestRedirect,
				RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
					Path: &gatewayv1.HTTPPathModifier{
						Type:            gatewayv1.FullPathHTTPPathModifier,
						ReplaceFullPath: new(defaultPath),
					},
					StatusCode: new(302),
				},
			},
		},
	}

	httpRoute.Spec.Rules = append([]gatewayv1.HTTPRouteRule{redirectRule}, httpRoute.Spec.Rules...)
}

func convertParentRefs(refs []configuration.HTTPRouteParentRef, _ string) []gatewayv1.ParentReference {
	if len(refs) == 0 {
		return nil
	}

	result := make([]gatewayv1.ParentReference, 0, len(refs))
	for _, ref := range refs {
		parentRef := gatewayv1.ParentReference{
			Name: gatewayv1.ObjectName(ref.Name),
		}

		if ref.Namespace != "" {
			parentRef.Namespace = new(gatewayv1.Namespace(ref.Namespace))
		}

		if ref.SectionName != "" {
			parentRef.SectionName = new(gatewayv1.SectionName(ref.SectionName))
		}

		result = append(result, parentRef)
	}

	return result
}
