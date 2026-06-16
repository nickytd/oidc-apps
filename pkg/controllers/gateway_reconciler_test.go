// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"reflect"
	"testing"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nickytd/oidc-apps/pkg/configuration"
)

func TestBuildInfrastructure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   *configuration.GatewayInfrastructure
		want *gatewayv1.GatewayInfrastructure
	}{
		{
			name: "nil input returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "annotations only",
			in: &configuration.GatewayInfrastructure{
				Annotations: map[string]string{
					"external-dns.alpha.kubernetes.io/hostname": "oidc-apps.example.com",
				},
			},
			want: &gatewayv1.GatewayInfrastructure{
				Annotations: map[gatewayv1.AnnotationKey]gatewayv1.AnnotationValue{
					"external-dns.alpha.kubernetes.io/hostname": "oidc-apps.example.com",
				},
			},
		},
		{
			name: "labels only",
			in: &configuration.GatewayInfrastructure{
				Labels: map[string]string{
					"team": "platform",
				},
			},
			want: &gatewayv1.GatewayInfrastructure{
				Labels: map[gatewayv1.LabelKey]gatewayv1.LabelValue{
					"team": "platform",
				},
			},
		},
		{
			name: "parametersRef only",
			in: &configuration.GatewayInfrastructure{
				ParametersRef: &configuration.GatewayParametersRef{
					Group: "",
					Kind:  "ConfigMap",
					Name:  "oidc-apps-gateway-svc-params",
				},
			},
			want: &gatewayv1.GatewayInfrastructure{
				ParametersRef: &gatewayv1.LocalParametersReference{
					Group: "",
					Kind:  "ConfigMap",
					Name:  "oidc-apps-gateway-svc-params",
				},
			},
		},
		{
			name: "full struct",
			in: &configuration.GatewayInfrastructure{
				Annotations: map[string]string{"a": "1"},
				Labels:      map[string]string{"l": "2"},
				ParametersRef: &configuration.GatewayParametersRef{
					Group: "example.com",
					Kind:  "GatewayParams",
					Name:  "params",
				},
			},
			want: &gatewayv1.GatewayInfrastructure{
				Annotations: map[gatewayv1.AnnotationKey]gatewayv1.AnnotationValue{"a": "1"},
				Labels:      map[gatewayv1.LabelKey]gatewayv1.LabelValue{"l": "2"},
				ParametersRef: &gatewayv1.LocalParametersReference{
					Group: "example.com",
					Kind:  "GatewayParams",
					Name:  "params",
				},
			},
		},
		{
			name: "empty annotations and labels stay nil",
			in: &configuration.GatewayInfrastructure{
				Annotations: map[string]string{},
				Labels:      map[string]string{},
			},
			want: &gatewayv1.GatewayInfrastructure{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := buildInfrastructure(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("buildInfrastructure() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
