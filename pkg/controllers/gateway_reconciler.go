// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
)

const syncInterval = 60 * time.Second

// GatewayReconciler reconciles a managed Gateway resource
type GatewayReconciler struct {
	Client    client.Client
	Namespace string
	Config    func() *configuration.GatewayGlobalConf
}

// Reconcile ensures the managed Gateway matches the desired configuration
func (g *GatewayReconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	conf := g.Config()
	if conf == nil || !conf.Managed {
		if err := g.cleanupOrphanedGateway(ctx); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	if err := g.reconcileGateway(ctx, conf); err != nil {
		log.FromContext(ctx).Error(err, "Failed to reconcile gateway")

		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: syncInterval}, nil
}

func (g *GatewayReconciler) cleanupOrphanedGateway(ctx context.Context) error {
	gwList := &gatewayv1.GatewayList{}
	if err := g.Client.List(ctx, gwList,
		client.InNamespace(g.Namespace),
		client.MatchingLabels{constants.LabelKey: constants.LabelValue},
	); err != nil {
		return fmt.Errorf("failed to list gateways: %w", err)
	}

	for i := range gwList.Items {
		log.FromContext(ctx).Info("Cleaning up orphaned managed gateway", "name", gwList.Items[i].Name)

		if err := g.Client.Delete(ctx, &gwList.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete orphaned gateway %s: %w", gwList.Items[i].Name, err)
		}
	}

	return nil
}

func (g *GatewayReconciler) reconcileGateway(ctx context.Context, conf *configuration.GatewayGlobalConf) error {
	if conf.GatewayClassName == "" {
		return errors.New("gatewayClassName is required when managed gateway is enabled")
	}

	name := conf.Name
	if name == "" {
		name = constants.ManagedGatewayName
	}

	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, g.Client, gw, func() error {
		gw.Labels = map[string]string{
			constants.LabelKey: constants.LabelValue,
		}
		maps.Copy(gw.Labels, conf.Labels)

		gw.Annotations = conf.Annotations
		gw.Spec = gatewayv1.GatewaySpec{
			GatewayClassName: gatewayv1.ObjectName(conf.GatewayClassName),
			Listeners:        buildListeners(conf.Listeners),
			Infrastructure:   buildInfrastructure(conf.Infrastructure),
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile gateway: %w", err)
	}

	if result != controllerutil.OperationResultNone {
		log.FromContext(ctx).Info("Reconciled managed gateway", "name", name, "namespace", g.Namespace, "operation", result)
	}

	return nil
}

func buildListeners(listeners []configuration.GatewayListener) []gatewayv1.Listener {
	result := make([]gatewayv1.Listener, 0, len(listeners))

	for _, l := range listeners {
		listener := gatewayv1.Listener{
			Name:     gatewayv1.SectionName(l.Name),
			Port:     gatewayv1.PortNumber(l.Port),
			Protocol: gatewayv1.ProtocolType(l.Protocol),
		}

		if l.Hostname != "" {
			hostname := gatewayv1.Hostname(l.Hostname)
			listener.Hostname = &hostname
		}

		if l.TLS != nil {
			listener.TLS = buildListenerTLS(l.TLS)
		}

		if l.AllowedRoutes != nil {
			listener.AllowedRoutes = buildAllowedRoutes(l.AllowedRoutes)
		}

		result = append(result, listener)
	}

	return result
}

func buildListenerTLS(tls *configuration.ListenerTLSConfig) *gatewayv1.ListenerTLSConfig {
	config := &gatewayv1.ListenerTLSConfig{}

	if tls.Mode != "" {
		mode := gatewayv1.TLSModeType(tls.Mode)
		config.Mode = &mode
	}

	for _, ref := range tls.CertificateRefs {
		group := gatewayv1.Group(ref.Group)

		kind := gatewayv1.Kind(ref.Kind)
		if kind == "" {
			kind = "Secret"
		}

		secretRef := gatewayv1.SecretObjectReference{
			Name:  gatewayv1.ObjectName(ref.Name),
			Group: &group,
			Kind:  &kind,
		}

		if ref.Namespace != "" {
			ns := gatewayv1.Namespace(ref.Namespace)
			secretRef.Namespace = &ns
		}

		config.CertificateRefs = append(config.CertificateRefs, secretRef)
	}

	if len(tls.Options) > 0 {
		config.Options = make(map[gatewayv1.AnnotationKey]gatewayv1.AnnotationValue, len(tls.Options))
		for k, v := range tls.Options {
			config.Options[gatewayv1.AnnotationKey(k)] = gatewayv1.AnnotationValue(v)
		}
	}

	return config
}

func buildAllowedRoutes(ar *configuration.AllowedRoutes) *gatewayv1.AllowedRoutes {
	routes := &gatewayv1.AllowedRoutes{}

	if ar.Namespaces != nil {
		routes.Namespaces = &gatewayv1.RouteNamespaces{}

		if ar.Namespaces.From != "" {
			from := gatewayv1.FromNamespaces(ar.Namespaces.From)
			routes.Namespaces.From = &from
		}

		if ar.Namespaces.Selector != nil {
			routes.Namespaces.Selector = ar.Namespaces.Selector
		}
	}

	for _, k := range ar.Kinds {
		rgk := gatewayv1.RouteGroupKind{
			Kind: gatewayv1.Kind(k.Kind),
		}

		if k.Group != "" {
			group := gatewayv1.Group(k.Group)
			rgk.Group = &group
		}

		routes.Kinds = append(routes.Kinds, rgk)
	}

	return routes
}

// buildInfrastructure converts the chart-facing GatewayInfrastructure config to
// the upstream Gateway-API type. Returns nil for nil input so the reconciler's
// CreateOrUpdate mutate fn doesn't diff nil-vs-empty-struct on every pass.
func buildInfrastructure(in *configuration.GatewayInfrastructure) *gatewayv1.GatewayInfrastructure {
	if in == nil {
		return nil
	}

	infra := &gatewayv1.GatewayInfrastructure{}

	if len(in.Annotations) > 0 {
		infra.Annotations = make(map[gatewayv1.AnnotationKey]gatewayv1.AnnotationValue, len(in.Annotations))
		for k, v := range in.Annotations {
			infra.Annotations[gatewayv1.AnnotationKey(k)] = gatewayv1.AnnotationValue(v)
		}
	}

	if len(in.Labels) > 0 {
		infra.Labels = make(map[gatewayv1.LabelKey]gatewayv1.LabelValue, len(in.Labels))
		for k, v := range in.Labels {
			infra.Labels[gatewayv1.LabelKey(k)] = gatewayv1.LabelValue(v)
		}
	}

	if in.ParametersRef != nil {
		infra.ParametersRef = &gatewayv1.LocalParametersReference{
			Group: gatewayv1.Group(in.ParametersRef.Group),
			Kind:  gatewayv1.Kind(in.ParametersRef.Kind),
			Name:  in.ParametersRef.Name,
		}
	}

	return infra
}
