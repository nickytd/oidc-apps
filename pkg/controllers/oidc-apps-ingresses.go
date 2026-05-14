// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"fmt"
	"maps"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
	"github.com/nickytd/oidc-apps/pkg/randutils"
)

func ingressForDeploymentObject(object client.Object) *networkingv1.Ingress {
	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.IngressName + "-" + suffix,
			Namespace: object.GetNamespace(),
		},
	}
}

func mutateIngressForDeployment(ingress *networkingv1.Ingress, object client.Object) error {
	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())
	extConfig := configuration.GetOIDCAppsControllerConfig()
	ingressClassName := new(extConfig.GetIngressClassName(object))
	ingressTLSSecretName := extConfig.GetIngressTLSSecretName(object)
	host := extConfig.GetHost(object)

	ingress.Labels = map[string]string{
		constants.LabelKey: constants.LabelValue,
	}
	ingress.Spec = networkingv1.IngressSpec{
		IngressClassName: ingressClassName,
		TLS: []networkingv1.IngressTLS{
			{
				Hosts:      []string{host},
				SecretName: ingressTLSSecretName,
			},
		},
		Rules: []networkingv1.IngressRule{
			{
				Host: host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: new(networkingv1.PathTypePrefix),
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: constants.ServiceNameOauth2Service + "-" + suffix,
										Port: networkingv1.ServiceBackendPort{
											Name: "http",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if annotations := extConfig.GetIngressAnnotations(object); len(annotations) > 0 {
		ingress.Annotations = annotations
	}

	applyIngressDefaultPathRedirect(ingress, object)

	extraLabels := extConfig.GetIngressLabels(object)
	maps.Copy(ingress.Labels, extraLabels)

	return nil
}

func ingressForStatefulSetPodObject(pod *corev1.Pod, object client.Object) *networkingv1.Ingress {
	suffix := randutils.GenerateSha256(pod.GetName() + "-" + pod.GetNamespace())
	index := fetchStrIndexIfPresent(pod)

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.IngressName + "-" + addOptionalIndex(index+"-") + suffix,
			Namespace: object.GetNamespace(),
		},
	}
}

func mutateIngressForStatefulSetPod(ingress *networkingv1.Ingress, pod *corev1.Pod, object client.Object) error {
	suffix := randutils.GenerateSha256(pod.GetName() + "-" + pod.GetNamespace())
	extConfig := configuration.GetOIDCAppsControllerConfig()
	ingressClassName := new(extConfig.GetIngressClassName(object))
	ingressTLSSecretName := extConfig.GetIngressTLSSecretName(object)
	index := fetchStrIndexIfPresent(pod)

	hostPrefix, ok := pod.GetAnnotations()[constants.AnnotationHostKey]
	if !ok {
		return fmt.Errorf("host annotation not found in pod %s/%s", pod.GetNamespace(), pod.GetName())
	}

	host, domain, _ := strings.Cut(hostPrefix, ".")
	podHost := fmt.Sprintf("%s-%s.%s", host, fetchStrIndexIfPresent(pod), domain)

	ingress.Labels = map[string]string{
		constants.LabelKey: constants.LabelValue,
	}
	ingress.Spec = networkingv1.IngressSpec{
		IngressClassName: ingressClassName,
		TLS: []networkingv1.IngressTLS{
			{
				Hosts:      []string{podHost},
				SecretName: ingressTLSSecretName,
			},
		},
		Rules: []networkingv1.IngressRule{
			{
				Host: podHost,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: new(networkingv1.PathTypePrefix),
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: constants.ServiceNameOauth2Service + "-" + addOptionalIndex(index+"-") + suffix,
										Port: networkingv1.ServiceBackendPort{
											Name: "http",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if annotations := extConfig.GetIngressAnnotations(object); len(annotations) > 0 {
		ingress.Annotations = annotations
	}

	applyIngressDefaultPathRedirect(ingress, object)

	extraLabels := extConfig.GetIngressLabels(object)
	maps.Copy(ingress.Labels, extraLabels)

	return nil
}

func applyIngressDefaultPathRedirect(ingress *networkingv1.Ingress, object client.Object) {
	defaultPath := configuration.GetOIDCAppsControllerConfig().GetIngressDefaultPath(object)
	key := "nginx.ingress.kubernetes.io/configuration-snippet"

	if defaultPath == "" {
		delete(ingress.Annotations, key)

		return
	}

	if ingress.Annotations == nil {
		ingress.Annotations = make(map[string]string)
	}

	ingress.Annotations[key] = fmt.Sprintf("rewrite ^/$ %s redirect;", defaultPath)
}
