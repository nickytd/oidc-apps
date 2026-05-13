// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"errors"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nickytd/oidc-apps/pkg/constants"
	"github.com/nickytd/oidc-apps/pkg/randutils"
)

var _ = Describe("Managed Gateway Test", Ordered, func() {
	Context("when the managed gateway is enabled", Ordered, func() {
		It("the controller shall create a managed Gateway resource", func(ctx SpecContext) {
			Eventually(func() error {
				gw := &gatewayv1.Gateway{}
				if err := clt.Get(ctx, client.ObjectKey{
					Name:      "test-managed-gateway",
					Namespace: defaultNamespace,
				}, gw); err != nil {
					return err
				}

				if gw.Labels[constants.LabelKey] != constants.LabelValue {
					return errors.New("managed gateway is missing oidc-apps label")
				}

				if string(gw.Spec.GatewayClassName) != "test-gateway-class" {
					return errors.New("managed gateway has incorrect gatewayClassName")
				}

				if len(gw.Spec.Listeners) != 1 {
					return errors.New("managed gateway should have exactly 1 listener")
				}

				listener := gw.Spec.Listeners[0]
				if string(listener.Name) != "https" {
					return errors.New("listener name should be 'https'")
				}

				if listener.Port != 443 {
					return errors.New("listener port should be 443")
				}

				if string(listener.Protocol) != "HTTPS" {
					return errors.New("listener protocol should be HTTPS")
				}

				if listener.Hostname == nil || string(*listener.Hostname) != "*.example.com" {
					return errors.New("listener hostname should be '*.example.com'")
				}

				if listener.AllowedRoutes == nil ||
					listener.AllowedRoutes.Namespaces == nil ||
					*listener.AllowedRoutes.Namespaces.From != gatewayv1.NamespacesFromAll {
					return errors.New("listener allowedRoutes should allow all namespaces")
				}

				return nil
			}).WithPolling(100 * time.Millisecond).WithTimeout(5 * time.Second).Should(Succeed())
		})
	})

	Context("when a deployment is a target with HTTPRoute and no explicit parentRefs", Ordered, func() {
		BeforeAll(func(ctx SpecContext) {
			deployment := createManagedGatewayHTTPRouteTargetDeployment()
			Eventually(func() error {
				return clt.Create(ctx, deployment)
			}).WithPolling(100 * time.Millisecond).Should(Succeed())

			replicaSet := createReplicaSet(deployment)
			Eventually(func() error {
				return clt.Create(ctx, replicaSet)
			}).WithPolling(100 * time.Millisecond).Should(Succeed())

			pod := createPod(replicaSet)
			Eventually(func() error {
				return clt.Create(ctx, pod)
			}).WithPolling(100 * time.Millisecond).Should(Succeed())
		}, NodeTimeout(5*time.Second))

		AfterAll(func(ctx SpecContext) {
			cleanUpAllDeployments(ctx)
		}, NodeTimeout(5*time.Second))

		It("the HTTPRoute shall have parentRefs auto-populated with the managed gateway", func(ctx SpecContext) {
			suffix := randutils.GenerateSha256(strings.Join([]string{managedGatewayHTTPRouteTarget, defaultNamespace}, "-"))

			Eventually(func() error {
				httpRoutes := gatewayv1.HTTPRouteList{}

				if err := clt.List(ctx, &httpRoutes,
					client.InNamespace(defaultNamespace),
					client.MatchingLabels{constants.LabelKey: constants.LabelValue},
				); err != nil {
					return err
				}

				for _, httpRoute := range httpRoutes.Items {
					if httpRoute.Name != strings.Join([]string{constants.HTTPRouteName, suffix}, "-") {
						continue
					}

					if len(httpRoute.Spec.ParentRefs) == 0 {
						return errors.New("HTTPRoute should have parentRefs")
					}

					parentRef := httpRoute.Spec.ParentRefs[0]
					if string(parentRef.Name) != "test-managed-gateway" {
						return errors.New("parentRef name should be 'test-managed-gateway'")
					}

					if parentRef.Namespace == nil || string(*parentRef.Namespace) != defaultNamespace {
						return errors.New("parentRef namespace should be 'default'")
					}

					return nil
				}

				return errors.New("HTTPRoute not found")
			}).WithPolling(100 * time.Millisecond).WithTimeout(5 * time.Second).Should(Succeed())
		})
	})
})
