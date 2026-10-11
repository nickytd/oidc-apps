// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolvedConfig is an immutable snapshot of the configuration resolved for a
// single workload object. The matching target is resolved exactly once (via
// Resolve) and all object-scoped accessors read from this snapshot, avoiding
// the per-field target re-matching (and namespace API lookups) that the
// accessors on OIDCAppsControllerConfig would otherwise incur.
type ResolvedConfig struct {
	target Target
	global Global
	object client.Object
	log    logr.Logger
}

// Resolve matches the given object against the configured targets once and
// returns a snapshot bound to that object. Accessors on the returned
// ResolvedConfig never re-match the object.
func (c *OIDCAppsControllerConfig) Resolve(ctx context.Context, object client.Object) *ResolvedConfig {
	return &ResolvedConfig{
		target: c.FetchTarget(ctx, object),
		global: c.Global,
		object: object,
		log:    c.log,
	}
}
