// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Match accepts a client.Object and verifies if is a target defined in the controller configuration
func (c *OIDCAppsControllerConfig) Match(ctx context.Context, o client.Object) bool {
	if c == nil || len(c.Targets) == 0 {
		return false
	}

	for _, t := range c.Targets {
		if c.targetMatchesLabels(ctx, t, o) {
			return true
		}
	}

	return false
}

// FetchTarget fetches the target associated with the given object.
func (c *OIDCAppsControllerConfig) FetchTarget(ctx context.Context, o client.Object) Target {
	var targets []Target

	for _, t := range c.Targets {
		target := t
		if c.targetMatchesLabels(ctx, target, o) {
			targets = append(targets, target)
		}
	}

	if len(targets) > 1 {
		c.log.Info("Multiple targets are fetched", "count", len(targets), "object", o.GetNamespace()+"/"+o.GetName())
	}

	if len(targets) > 0 {
		return targets[0]
	}

	return Target{}
}

func (c *OIDCAppsControllerConfig) targetMatchesLabels(ctx context.Context, t Target, o client.Object) bool {
	selector, err := metav1.LabelSelectorAsSelector(t.LabelSelector)
	if err != nil {
		c.logMatchError(err, "failed to parse target label selector", t, o)

		return false
	}

	if t.NamespaceSelector.Size() == 0 {
		return selector.Matches(labels.Set(o.GetLabels()))
	}

	if c.client == nil {
		return false
	}

	namespace := &corev1.Namespace{}

	if err = c.client.Get(ctx, client.ObjectKey{Name: o.GetNamespace()}, namespace); err != nil {
		c.logMatchError(err, "failed to get namespace for target match", t, o)

		return false
	}

	namespaceSelector, err := metav1.LabelSelectorAsSelector(t.NamespaceSelector)
	if err != nil {
		c.logMatchError(err, "failed to parse target namespace selector", t, o)

		return false
	}

	if t.LabelSelector.Size() == 0 {
		return namespaceSelector.Matches(labels.Set(namespace.GetLabels()))
	}

	return selector.Matches(labels.Set(o.GetLabels())) && namespaceSelector.Matches(labels.Set(namespace.GetLabels()))
}

// logMatchError logs matching failures without masking them as silent non-matches.
func (c *OIDCAppsControllerConfig) logMatchError(err error, message string, t Target, o client.Object) {
	if c.log.IsZero() {
		return
	}

	c.log.Error(err, message, "target", t.Name, "object", o.GetNamespace()+"/"+o.GetName())
}

// GetTargetLabelSelector returns the label selector for the given target
func (c *OIDCAppsControllerConfig) GetTargetLabelSelector(ctx context.Context, o client.Object) *metav1.LabelSelector {
	t := c.FetchTarget(ctx, o)

	return t.LabelSelector
}
