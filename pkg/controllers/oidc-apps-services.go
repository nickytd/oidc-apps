// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nickytd/oidc-apps/pkg/constants"
	"github.com/nickytd/oidc-apps/pkg/randutils"
)

func oauth2ServiceObject(object client.Object) *corev1.Service {
	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())
	index := fetchStrIndexIfPresent(object)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.ServiceNameOauth2Service + "-" + addOptionalIndex(index+"-") + suffix,
			Namespace: object.GetNamespace(),
		},
	}
}

func mutateOauth2Service(svc *corev1.Service, selectors client.MatchingLabels) error {
	svc.Labels = map[string]string{constants.LabelKey: constants.LabelValue}
	svc.Spec = corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       "http",
				Port:       8080,
				TargetPort: intstr.FromString("oauth2"),
			},
		},
		Selector: selectors,
	}

	return nil
}

func fetchStrIndexIfPresent(object client.Object) string {
	idx, present := object.GetLabels()["statefulset.kubernetes.io/pod-name"]
	if present {
		l := strings.Split(idx, "-")

		return l[len(l)-1]
	}

	return ""
}
