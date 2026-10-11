# Drop Ingress Support — Gateway API Only — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all Kubernetes `networking.k8s.io/Ingress` support from the oidc-apps-controller, leaving Gateway API `HTTPRoute` as the only routing entry-point.

**Architecture:** Delete the Ingress analog of every Gateway construct in place, following the existing parallel file layout. Make Gateway API/HTTPRoute wiring unconditional (remove the `httpRoutes.enabled` global gate), keep the managed-Gateway feature as an independent opt-in, and merge the two host-resolution functions into a single `GetHost()` that reads `target.HTTPRoute`.

**Tech Stack:** Go 1.26, controller-runtime, Gateway API (`sigs.k8s.io/gateway-api/apis/v1`), Ginkgo/Gomega + envtest, Helm.

## Global Constraints

- License header on every Go source file (SPDX, copied from existing files): `// SPDX-FileCopyrightText: 2026 nickytd` / `// SPDX-License-Identifier: Apache-2.0`.
- Import order enforced by gci: stdlib, third-party, then `github.com/nickytd/oidc-apps`. Run `make gci`.
- Conventional Commits: `<type>(<scope>): <subject>`. Separate logical changes into separate commits.
- Config is decoded with non-strict `sigs.k8s.io/yaml` — leftover `ingress:` blocks are silently ignored, not rejected. This is the documented breaking behavior.
- Gateway API CRDs (`HTTPRoute`; `Gateway` when managed) must be present in every cluster, because the scheme/cache/watches are now unconditional.
- After each code change to `.go` files, run `make fmt gci` then `go build ./...` before committing.

---

## File Structure

Files deleted:
- `pkg/configuration/ingress.go` (getters relocated/dropped; `isValidDefaultPath` moved to `general.go`)
- `pkg/controllers/oidc-apps-ingresses.go` (all Ingress builders/mutators)

Files modified:
- `pkg/configuration/types.go` — drop `Target.Ingress`, `IngressConf`
- `pkg/configuration/general.go` — rework `GetHost()`, home for `isValidDefaultPath`
- `pkg/configuration/gateway.go` — drop `GetHTTPRouteHost`, `IsHTTPRouteEnabled`, de-gate
- `pkg/controllers/modifiers.go` — drop Ingress fetch/reconcile; de-gate HTTPRoute fetch
- `pkg/controllers/owned.go` — drop Ingress cleanup
- `pkg/oidc-apps/cmd.go` — drop Ingress wiring, de-gate HTTPRoute wiring
- `pkg/webhook/modifiers.go` — delete `resolveHost`
- `pkg/webhook/pod-webhook.go` — call `rc.GetHost()` directly; fix stale comment
- `pkg/constants/const.go` — drop `IngressName`
- `charts/oidc-apps/templates/clusterrole.yaml`, `values.yaml`, `values.schema.json`, `Chart.yaml`
- Tests: `pkg/configuration/config_test.go`, `pkg/configuration/test/configuration.yaml`, `pkg/controllers/oidc-apps-ingresses_test.go`, `test/e2e/{suite_test,setup,deployment_test,statefulset_test}.go`
- Docs/examples: `example/config.yaml`, `example/kind-setup/README.md`, `README.md`, new `CHANGELOG.md`

---

## Task 1: Rework `GetHost()` and relocate the default-path helper (config)

This must come first: `GetHost()` currently depends on `target.Ingress`, so it has to switch to `target.HTTPRoute` before `IngressConf` can be deleted. `isValidDefaultPath` lives in `ingress.go` (which Task 2 deletes) but is used by `GetHTTPRouteDefaultPath`, so it moves to `general.go` here.

**Files:**
- Modify: `pkg/configuration/general.go`
- Modify: `pkg/configuration/config_test.go` (host tests + redirect-URL test)
- Modify: `pkg/configuration/test/configuration.yaml` (host fixtures)

**Interfaces:**
- Produces: `(rc *ResolvedConfig) GetHost() string` — now reads `rc.target.HTTPRoute`. `isValidDefaultPath(path string) bool`, `validDefaultPathRe`, `maxDefaultPathLength` now defined in `general.go`.
- Consumes: `rc.target.HTTPRoute *HTTPRouteConf` (existing).

- [ ] **Step 1: Update host test fixtures to use httpRoute**

In `pkg/configuration/test/configuration.yaml`, the host tests target `test-02` (hostPrefix), `test-03` (host override), `test-04` (no routing config). These currently use `ingress:`. Change the `ingress:` keys to `httpRoute:` for the three host-test targets so `GetHost()` has data to read after the rework. Edit test-02 (lines 38–42):

```yaml
    httpRoute:
      create: true
      hostPrefix: "test-02-prefix"
      tlsSecretRef:
        name: "ingress-tls"
```

Note: `httpRoute` has no `tlsSecretRef` field — drop it. Final test-02 routing block:

```yaml
    httpRoute:
      create: true
      hostPrefix: "test-02-prefix"
```

Edit test-03 (lines 75–76):

```yaml
    httpRoute:
      host: "this.overwrites"
```

test-04 keeps no routing block (unchanged). Leave test-05…test-12 as-is.

- [ ] **Step 2: Rewrite `GetHost()` to read `target.HTTPRoute` and add the helper**

Replace the body of `pkg/configuration/general.go` with (keeping `GetUpstreamTarget` unchanged, adding `regexp` import and the relocated helper):

```go
// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"regexp"
	"strings"

	"github.com/nickytd/oidc-apps/pkg/randutils"
)

// GetHost return the domain name for a given workload target
func (rc *ResolvedConfig) GetHost() string {
	domain := rc.global.DomainName

	prefix := rc.object.GetName() + "-" + rc.object.GetNamespace()
	if rc.target.HTTPRoute != nil && rc.target.HTTPRoute.HostPrefix != "" {
		prefix = rc.target.HTTPRoute.HostPrefix + "-" + randutils.GenerateSha256(rc.object.GetName()+"-"+rc.object.GetNamespace())
	}

	if rc.target.HTTPRoute != nil && rc.target.HTTPRoute.Host != "" {
		prefix, domain, _ = strings.Cut(rc.target.HTTPRoute.Host, ".")
	}

	if domain == "" {
		return prefix
	}

	return strings.Join([]string{prefix, domain}, ".")
}

// GetUpstreamTarget returns the protocol and port tuple of the target workload
func (rc *ResolvedConfig) GetUpstreamTarget() string {
	b := strings.Builder{}
	protocol := "http"

	if rc.target.TargetProtocol == "https" {
		protocol = "https"
	}

	b.Grow(9)
	_, _ = b.WriteString("protocol=")
	b.Grow(len(protocol))
	_, _ = b.WriteString(protocol)
	b.Grow(7)
	_, _ = b.WriteString(", port=")
	b.Grow(len(rc.target.TargetPort.String()))
	_, _ = b.WriteString(rc.target.TargetPort.String())

	return b.String()
}

const maxDefaultPathLength = 32

var validDefaultPathRe = regexp.MustCompile(`^/[-a-zA-Z0-9/_.~]+$`)

func isValidDefaultPath(path string) bool {
	return len(path) <= maxDefaultPathLength && validDefaultPathRe.MatchString(path)
}
```

- [ ] **Step 3: Rename and keep the host tests; update the redirect-URL test**

In `pkg/configuration/config_test.go`:

Rename `TestTargetIngressHostPrefix` → `TestTargetHostPrefix`, `TestTargetIngressHost` → `TestTargetHost`, `TestTargetWithoutIngressHost` → `TestTargetWithoutHost`. Their bodies are unchanged — they call `GetHost()` and assert the same expected strings (test-02 prefix `test-02-prefix-`, test-03 `this.overwrites`, test-04 `test-04-test.domain.org`), which still hold because the fixtures moved the same values under `httpRoute:`.

Update `TestGetRedirectURLWithHTTPRoute`: its comment claims "GetHost uses ingress.Host (not httpRoute.Host), so it falls back to default" and asserts `https://test-06-test.domain.org/oauth2/callback`. Now that `GetHost()` reads `httpRoute.Host`, test-06 (`host: custom.httproute.host`) resolves differently. Replace the comment and assertion:

```go
	// GetRedirectURL falls through to https://{GetHost}/oauth2/callback.
	// GetHost now reads httpRoute.Host, so test-06's custom host is used.
	// strings.Cut("custom.httproute.host", ".") → prefix="custom", domain="httproute.host"
	redirectURL := extensionConfig.Resolve(t.Context(), target).GetRedirectURL()
	g.Expect(redirectURL).To(Equal("https://custom.httproute.host/oauth2/callback"))
```

(Rationale: `strings.Cut("custom.httproute.host", ".")` returns everything after the FIRST separator — prefix `custom`, domain `httproute.host` — so `GetHost()` rejoins them to `custom.httproute.host` and `GetOidcRedirectURL` wraps it as `https://custom.httproute.host/oauth2/callback`.)

- [ ] **Step 4: Run the configuration unit tests**

Run: `go test ./pkg/configuration/... -run 'TestTargetHost|TestTargetHostPrefix|TestTargetWithoutHost|TestGetRedirectURLWithHTTPRoute' -v`
Expected: PASS. (Other ingress tests in this file still reference deleted getters and are handled in Task 2 — if the package doesn't compile yet because `ingress.go` still exists, that's fine; `ingress.go` is untouched in this task and still provides those getters, so the whole package should still build and this run should pass.)

- [ ] **Step 5: Build and commit**

Run: `make fmt gci && go build ./...`
Expected: no errors.

```bash
git add pkg/configuration/general.go pkg/configuration/config_test.go pkg/configuration/test/configuration.yaml
git commit -m "refactor(configuration): resolve host from httpRoute and relocate path helper"
```

---

## Task 2: Remove Ingress config types and getters; de-gate HTTPRoute config

**Files:**
- Delete: `pkg/configuration/ingress.go`
- Modify: `pkg/configuration/types.go`
- Modify: `pkg/configuration/gateway.go`
- Modify: `pkg/configuration/config_test.go` (remove ingress-getter tests)

**Interfaces:**
- Removes: `IngressConf`, `Target.Ingress`, all `GetIngress*`/`ShallCreateIngress` methods, `GetHTTPRouteHost`, `IsHTTPRouteEnabled`, `isHTTPRouteEnabled`.
- Produces: `(rc *ResolvedConfig) ShallCreateHTTPRoute() bool` (no longer gated on enabled); `(c *OIDCAppsControllerConfig) IsManagedGatewayEnabled() bool` = `c.Global.Gateway != nil && c.Global.Gateway.Managed`.

- [ ] **Step 1: Delete `pkg/configuration/ingress.go`**

```bash
git rm pkg/configuration/ingress.go
```

(The `isValidDefaultPath` helper it held was relocated to `general.go` in Task 1.)

- [ ] **Step 2: Remove `Target.Ingress` and `IngressConf` from `types.go`**

In `pkg/configuration/types.go`, delete the `Ingress *IngressConf` field from the `Target` struct (line 69) and delete the entire `IngressConf` struct (lines 74–84). Leave `HTTPRoute *HTTPRouteConf` and `HTTPRouteConf` intact.

- [ ] **Step 3: De-gate HTTPRoute in `gateway.go`**

In `pkg/configuration/gateway.go`:

Delete `isHTTPRouteEnabled()` (lines 112–116) and `IsHTTPRouteEnabled()` (lines 142–148).

Delete `GetHTTPRouteHost()` (lines 160–178) — `GetHost()` now serves this purpose.

Rewrite `isManagedGatewayEnabled()` (ResolvedConfig) to not depend on the removed helper:

```go
// isManagedGatewayEnabled returns true if the controller should create and manage a Gateway resource
func (rc *ResolvedConfig) isManagedGatewayEnabled() bool {
	return rc.global.Gateway != nil && rc.global.Gateway.Managed
}
```

Rewrite `ShallCreateHTTPRoute()` to drop the enabled gate (keep the empty-parentRefs warning):

```go
// ShallCreateHTTPRoute returns true if the target workload shall create an HTTPRoute
func (rc *ResolvedConfig) ShallCreateHTTPRoute() bool {
	if rc.target.HTTPRoute != nil {
		if rc.target.HTTPRoute.Create && len(rc.target.HTTPRoute.ParentRefs) == 0 && !rc.isManagedGatewayEnabled() {
			rc.log.Info("WARNING: HTTPRoute configuration has empty parentRefs - HTTPRoute will not be attached to any Gateway",
				"target", rc.target.Name,
				"object", rc.object.GetNamespace()+"/"+rc.object.GetName())
		}

		return rc.target.HTTPRoute.Create
	}

	return false
}
```

Rewrite `IsManagedGatewayEnabled()` (OIDCAppsControllerConfig):

```go
// IsManagedGatewayEnabled returns true if the controller should create and manage a Gateway resource
func (c *OIDCAppsControllerConfig) IsManagedGatewayEnabled() bool {
	return c != nil && c.Global.Gateway != nil && c.Global.Gateway.Managed
}
```

- [ ] **Step 4: Remove ingress-getter tests from `config_test.go`**

Delete these test functions entirely: `TestGetIngressDefaultPath`, `TestGetIngressAnnotationsAndLabels`, `TestGetIngressClassName`. In `TestTargetGlobalKubeSecret` remove the line `g.Expect(rc.ShallCreateIngress()).To(BeFalse())`; in `TestTargetKubeSecret` remove `g.Expect(rc.ShallCreateIngress()).To(BeTrue())`; in `TestTargetGlobalConfiguration` remove `g.Expect(rc.ShallCreateIngress()).To(BeFalse())`. (The default-path/annotation/label behavior is already covered on the HTTPRoute side by the existing `GetHTTPRoute*` tests and fixtures test-09/test-11.)

- [ ] **Step 5: Build and test**

Run: `make fmt gci && go build ./... && go test ./pkg/configuration/... -v`
Expected: PASS, no unresolved references to `IngressConf`/`GetIngress*`/`IsHTTPRouteEnabled`.
(If `go build` reports callers of the removed symbols in `pkg/controllers`, `pkg/webhook`, or `pkg/oidc-apps`, that is expected — those are fixed in Tasks 3–5. To keep this task independently green, run the build scoped: `go build ./pkg/configuration/...`.)

- [ ] **Step 6: Commit**

```bash
git add pkg/configuration/
git commit -m "refactor(configuration): drop ingress config and ungate httproute"
```

---

## Task 3: Remove Ingress reconciliation and de-gate HTTPRoute fetch (controllers)

**Files:**
- Delete: `pkg/controllers/oidc-apps-ingresses.go`
- Modify: `pkg/controllers/modifiers.go`
- Modify: `pkg/controllers/owned.go`

**Interfaces:**
- Removes: `ingressForDeploymentObject`, `mutateIngressForDeployment`, `ingressForStatefulSetPodObject`, `mutateIngressForStatefulSetPod`, `applyIngressDefaultPathRedirect`, `fetchOidcAppsIngress`, `reconcileIngressForDeployment`, `reconcileIngressForStatefulSetPod`.
- Consumes: `rc.ShallCreateHTTPRoute()` (de-gated in Task 2).

- [ ] **Step 1: Delete the Ingress builder file**

```bash
git rm pkg/controllers/oidc-apps-ingresses.go
```

- [ ] **Step 2: Remove Ingress functions and calls from `modifiers.go`**

In `pkg/controllers/modifiers.go`:

Delete `fetchOidcAppsIngress` (lines 76–101), `reconcileIngressForDeployment` (lines 249–264), and `reconcileIngressForStatefulSetPod` (lines 266–282).

In `reconcileDeploymentDependencies`, delete the block (lines 234–236):

```go
	if err := reconcileIngressForDeployment(ctx, c, rc, object); err != nil {
		return err
	}

```

In `reconcileStatefulSetDependencies`, delete the block (lines 373–375):

```go
			if err := reconcileIngressForStatefulSetPod(ctx, c, rc, &pod, object); err != nil {
				return err
			}

```

De-gate `fetchOidcAppsHTTPRoutes` — remove the now-dangling `IsHTTPRouteEnabled` early return (lines 106–108):

```go
	if !cfg.IsHTTPRouteEnabled() {
		return &gatewayv1.HTTPRouteList{}, nil
	}

```

That early return was the function's only use of `cfg`. The repo's `.golangci.yaml` enables `revive` with `enable-all-rules: true`, whose `unused-parameter` rule is enforced (see the existing `fetchResourceAttributesNamespace(_ context.Context, _ client.Client, ...)` idiom in this same file). Rather than remove the parameter and cascade signature changes through `deleteOwnedResources` and its two reconciler call sites, blank it with `_` to match that idiom. Change the signature from:

```go
func fetchOidcAppsHTTPRoutes(ctx context.Context, c client.Client, cfg *configuration.OIDCAppsControllerConfig,
	object client.Object) (*gatewayv1.HTTPRouteList,
	error) {
```

to:

```go
func fetchOidcAppsHTTPRoutes(ctx context.Context, c client.Client, _ *configuration.OIDCAppsControllerConfig,
	object client.Object) (*gatewayv1.HTTPRouteList,
	error) {
```

All callers (`owned.go:61` passes `cfg`) and the `deleteOwnedResources` signature stay unchanged. Remove the `networkingv1 "k8s.io/api/networking/v1"` import (line 14).

- [ ] **Step 3: Remove Ingress cleanup from `owned.go`**

In `pkg/controllers/owned.go`, delete the ingress fetch+delete block in `deleteOwnedResources` (lines 48–59):

```go
	ingresses, err := fetchOidcAppsIngress(ctx, c, object)
	if err != nil {
		return err
	}

	for _, s := range ingresses.Items {
		if err = c.Delete(ctx, &s); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete ingress %s/%s: %w", s.Namespace, s.Name, err)
		}

		_log.V(9).Info("Deleted", "name", s.Name, "namespace", s.Namespace)
	}

```

- [ ] **Step 4: Build and run controller unit tests**

Run: `make fmt gci && go build ./pkg/controllers/... && go test ./pkg/controllers/... -v`
Expected: PASS. (`oidc-apps-ingresses_test.go` still compiles — it only tests `fetchStrIndexIfPresent`; it is renamed in Task 7.)

- [ ] **Step 5: Commit**

```bash
git add pkg/controllers/
git commit -m "refactor(controllers): remove ingress reconciliation, ungate httproute fetch"
```

---

## Task 4: Remove Ingress wiring and de-gate HTTPRoute wiring (manager)

**Files:**
- Modify: `pkg/oidc-apps/cmd.go`

**Interfaces:**
- Removes: `IngressMapFuncForStatefulset`, the `networkingv1` import, Ingress cache/index/watches, the `httpRouteEnabled` local.
- Gateway scheme install, HTTPRoute cache/index/watches become unconditional. Managed Gateway stays gated on `IsManagedGatewayEnabled()`.

- [ ] **Step 1: Remove the gate local and make the gateway scheme unconditional**

In `pkg/oidc-apps/cmd.go` `RunController`, delete lines 59–65:

```go
	// Load extension configuration first to determine HTTPRoute support
	extensionConfig = configuration.CreateControllerConfigOrDie(o.controllerConfigPath)
	httpRouteEnabled := extensionConfig.IsHTTPRouteEnabled()

	if httpRouteEnabled {
		_log.Info("Gateway API HTTPRoute support enabled via configuration")
	}
```

Replace with:

```go
	extensionConfig = configuration.CreateControllerConfigOrDie(o.controllerConfigPath)
```

Change the gateway scheme block (lines 82–87) from conditional to unconditional:

```go
	// Add Gateway API schemes for HTTPRoute support
	if err := gatewayv1.Install(sch); err != nil {
		return fmt.Errorf("could not initialize the gateway-api scheme: %w", err)
	}
```

- [ ] **Step 2: Remove the Ingress cache entry; make HTTPRoute cache unconditional**

In `cacheOptions.ByObject`, delete the `&networkingv1.Ingress{}` entry (lines 114–116):

```go
			&networkingv1.Ingress{}: {
				Label: labels.SelectorFromSet(labels.Set{constants.LabelKey: constants.LabelValue}),
			},
```

Replace the conditional HTTPRoute cache block (lines 125–130) with an unconditional one:

```go
	// HTTPRoute is always cached (Gateway API is the only routing path)
	cacheOptions.ByObject[&gatewayv1.HTTPRoute{}] = cache.ByObject{
		Label: labels.SelectorFromSet(labels.Set{constants.LabelKey: constants.LabelValue}),
	}
```

(Leave the `IsManagedGatewayEnabled()`-gated Gateway cache entry at 132–137 unchanged.)

- [ ] **Step 3: Remove the Ingress field index; make the HTTPRoute index unconditional**

In `initializeManagerIndices`, delete the entire `networkingv1.Ingress` index block (lines 310–331, the `err = mgr.GetFieldIndexer().IndexField(... &networkingv1.Ingress{} ...)` through its error check).

Change the HTTPRoute index block (lines 333–357) from `if extensionConfig.IsHTTPRouteEnabled() { ... }` to unconditional — unwrap the inner `IndexField` call so it always runs:

```go
	// HTTPRoute index is always registered
	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&gatewayv1.HTTPRoute{},
		"metadata.labels"+constants.LabelKey,
		func(obj client.Object) []string {
			httpRoute, ok := obj.(*gatewayv1.HTTPRoute)
			if !ok {
				_log.Error(errors.New("object is not an httproute"), "object", obj)

				return nil
			}

			if value, exists := httpRoute.GetLabels()[constants.LabelKey]; exists {
				return []string{value}
			}

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("could not set up the oidc-app-controller %T index: %w", gatewayv1.HTTPRoute{}, err)
	}

	return nil
```

(Ensure the function still ends with a single `return nil`.)

- [ ] **Step 4: Remove Ingress watches; make HTTPRoute watches unconditional**

In `addDeploymentController`, delete the Ingress `.Watches(...)` (lines 383–390):

```go
		Watches(
			&networkingv1.Ingress{},
			handler.EnqueueRequestForOwner(
				mgr.GetScheme(),
				mgr.GetRESTMapper(),
				&appsv1.Deployment{},
			),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
```

Then unwrap the conditional HTTPRoute watch (lines 396–406) so it is always chained. The cleanest form keeps the builder fluent — change the deployment builder's final `.Watches(&corev1.Pod{}, ...)` to be followed by the HTTPRoute watch and `.Complete(...)` unconditionally:

```go
func addDeploymentController(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named("oidc-apps-deployments").
		For(&appsv1.Deployment{}).
		WithEventFilter(fetchPredicates(extensionConfig)).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &appsv1.Deployment{}),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &appsv1.Deployment{}),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(PodMapFuncForDeployment(mgr)),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(
			&gatewayv1.HTTPRoute{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &appsv1.Deployment{}),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Complete(&controllers.DeploymentReconciler{Client: mgr.GetClient(), Config: extensionConfig})
}
```

In `addStatefulSetController`, delete the Ingress `.Watches(&networkingv1.Ingress{}, IngressMapFuncForStatefulset(mgr), ...)` (lines 436–439) and unwrap the HTTPRoute watch (441–447) the same way:

```go
func addStatefulSetController(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named("oidc-apps-statefulsets").
		For(&appsv1.StatefulSet{}).
		WithEventFilter(fetchPredicates(extensionConfig)).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &appsv1.StatefulSet{}),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &appsv1.StatefulSet{}),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(ServiceMapFuncForStatefulset(mgr)),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(
			&gatewayv1.HTTPRoute{},
			handler.EnqueueRequestsFromMapFunc(HTTPRouteMapFuncForStatefulset(mgr)),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Complete(&controllers.StatefulSetReconciler{Client: mgr.GetClient(), Config: extensionConfig})
}
```

- [ ] **Step 5: Delete `IngressMapFuncForStatefulset` and the import**

Delete the whole `IngressMapFuncForStatefulset` function (lines 598–644). Remove the `networkingv1 "k8s.io/api/networking/v1"` import (line 16).

- [ ] **Step 6: Build**

Run: `make fmt gci && go build ./...`
Expected: no errors across the whole module (Tasks 1–4 together make it compile).

- [ ] **Step 7: Commit**

```bash
git add pkg/oidc-apps/cmd.go
git commit -m "refactor(oidc-apps): remove ingress manager wiring, ungate httproute"
```

---

## Task 5: Simplify webhook host resolution

**Files:**
- Modify: `pkg/webhook/modifiers.go`
- Modify: `pkg/webhook/pod-webhook.go`

**Interfaces:**
- Removes: `resolveHost(cfg, rc)`.
- Consumes: `rc.GetHost()` (reworked in Task 1).

- [ ] **Step 1: Delete `resolveHost`**

In `pkg/webhook/modifiers.go`, delete the function (lines 493–499):

```go
func resolveHost(cfg *configuration.OIDCAppsControllerConfig, rc *configuration.ResolvedConfig) string {
	if cfg.IsHTTPRouteEnabled() {
		return rc.GetHTTPRouteHost()
	}

	return rc.GetHost()
}
```

- [ ] **Step 2: Replace the `resolveHost` call inside `addAnnotations` and blank its now-unused `cfg` parameter**

In `pkg/webhook/modifiers.go`, line 47 is inside `addAnnotations(target client.Object, cfg *configuration.OIDCAppsControllerConfig, rc *configuration.ResolvedConfig)`. Change the body line:

```go
	annotations[constants.AnnotationHostKey] = resolveHost(cfg, rc)
```

to:

```go
	annotations[constants.AnnotationHostKey] = rc.GetHost()
```

This leaves `cfg` unused. The repo enforces `revive`'s `unused-parameter`, so blank it with `_` (matching the `fetchResourceAttributesNamespace(_ context.Context, ...)` idiom) rather than removing it and touching the caller. Change the signature to:

```go
func addAnnotations(target client.Object, _ *configuration.OIDCAppsControllerConfig, rc *configuration.ResolvedConfig) {
```

The sole caller `addAnnotations(patch, p.Config, rc)` at `pod-webhook.go:75` stays unchanged.

- [ ] **Step 3: Replace the `resolveHost` calls in `pod-webhook.go`**

At line 80, change `resolveHost(p.Config, rc)` to `rc.GetHost()`. At line 149, change `hostPrefix := resolveHost(p.Config, rc)` to `hostPrefix := rc.GetHost()`.

Fix the stale comment at lines 144–147 — replace "the oidc-apps-controller creates a new ingress for each pod" with "the oidc-apps-controller creates a new HTTPRoute for each pod":

```go
	// When "present", the target pod is part of a statefulset.
	// In this case, the oidc-apps-controller creates a new HTTPRoute for each pod and the oauth2-proxy redirect
	// URL needs to reflect the target pod index. When the redirect URL is constructed the pod index is appended to the
	// host name.
```

- [ ] **Step 4: Build and run webhook tests**

Run: `make fmt gci && go build ./... && go test ./pkg/webhook/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/webhook/
git commit -m "refactor(webhook): resolve host via single GetHost"
```

---

## Task 6: Remove the `IngressName` constant

**Files:**
- Modify: `pkg/constants/const.go`

- [ ] **Step 1: Delete the constant**

In `pkg/constants/const.go`, delete lines 38–39:

```go
	// IngressName is the name of the oauth2 ingress
	IngressName = "oauth2-ingress"
```

Keep `HTTPRouteName` and `ManagedGatewayName`.

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: no errors. (If anything still references `constants.IngressName`, it is in the e2e tests — handled in Task 7. Scope the build to confirm non-test code is clean: `go build ./cmd/... ./pkg/...`.)

- [ ] **Step 3: Commit**

```bash
git add pkg/constants/const.go
git commit -m "refactor(constants): drop IngressName"
```

---

## Task 7: Update unit test for the renamed ingress test file

**Files:**
- Rename: `pkg/controllers/oidc-apps-ingresses_test.go` → `pkg/controllers/helpers_test.go`

The file only contains `TestFetchStrIndexIfPresent` (a shared pod-index helper test), despite its name. Rename so no file implies Ingress support remains.

- [ ] **Step 1: Rename the file (content unchanged)**

```bash
git mv pkg/controllers/oidc-apps-ingresses_test.go pkg/controllers/helpers_test.go
```

- [ ] **Step 2: Run the test**

Run: `go test ./pkg/controllers/... -run TestFetchStrIndexIfPresent -v`
Expected: PASS (all three sub-tests).

- [ ] **Step 3: Commit**

```bash
git add -A pkg/controllers/
git commit -m "test(controllers): rename misleading ingress test file"
```

---

## Task 8: Remove Ingress coverage from envtest and verify HTTPRoute parity

**Files:**
- Modify: `test/e2e/suite_test.go`
- Modify: `test/e2e/setup.go`
- Modify: `test/e2e/deployment_test.go`
- Modify: `test/e2e/statefulset_test.go`

**Interfaces:**
- Consumes: existing HTTPRoute e2e helpers in `httproute_test.go` / `gateway_test.go` (unchanged) for surviving coverage.

- [ ] **Step 1: Remove Ingress watches from `suite_test.go`**

In `test/e2e/suite_test.go`, in the deployment reconciler builder delete the Ingress `.Watches(&networkingv1.Ingress{}, EnqueueRequestForOwner, ...)` block (around lines 162–169). In the statefulset reconciler builder delete the Ingress `.Watches(&networkingv1.Ingress{}, oidcappscontroller.IngressMapFuncForStatefulset(mgr), ...)` block (around lines 209–213). Keep both `gatewayv1.HTTPRoute` watches. Remove the now-unused `networkingv1 "k8s.io/api/networking/v1"` import (line 21). (The core scheme already covers networking types; `gatewayv1.Install` at line 103 stays and is already unconditional.)

- [ ] **Step 2: Remove the Ingress assertion from `deployment_test.go`**

Delete the entire `It("there shall be an oidc-apps annotated ingress present in the deployment namespace", ...)` block (lines 69–124). The sidecar-container `It` immediately above it stays. If removing it leaves `networkingv1`, `maps`, or `randutils` imports unused in this file, remove those imports. Confirm an HTTPRoute-creation assertion for Deployments exists in `test/e2e/httproute_test.go`; if none covers the plain Deployment target, that gap is acceptable because `httproute_test.go` already exercises HTTPRoute creation for the configured targets — do not add new cases unless Step 6 shows a coverage hole.

- [ ] **Step 3: Remove the Ingress assertions and skip-ingress fixtures from `statefulset_test.go`**

Delete the `It("there shall be an oidc-apps ingress per pod present in the statefulset namespace", ...)` block (lines 71–161) and the `It("there shall be no oidc-apps ingress per pod present in the statefulset namespace", ...)` block (lines 318–348). Delete the surrounding skip-ingress `Context("when a statefulset is a target and ingress shall be skipped", Ordered, func() {...})` (starts line 274) in full, including its `BeforeAll`/`AfterAll` that call `createTargetSkipIngressStatefulSet()` and `createSkipIngressStatefulSetPod(...)`. Remove the teardown `clt.DeleteAllOf(ctx, &networkingv1.Ingress{}, deleteOptions...)` (line 526). Remove now-unused imports (`networkingv1`, and `strconv` if only the deleted negative block used it — verify).

- [ ] **Step 4: Remove skip-ingress helpers and constant from `setup.go`**

In `test/e2e/setup.go`, delete the `skipIngressTarget` const (line 24), the `skipIngressTarget` Deployment fixture entry (the struct element around lines 113–135), `createTargetSkipIngressStatefulSet()` (lines 341–371), and `createSkipIngressStatefulSetPod(...)` (lines 403–end of that function). Leave `createStatefulSetPod` and all other targets/fixtures intact.

- [ ] **Step 5: Build the e2e test binary**

Run: `go vet ./test/... && go build ./test/...`
Expected: no errors, no unused imports.

- [ ] **Step 6: Run envtest**

Run: `make envtest`
Expected: PASS. The suite now exercises only HTTPRoute routing. If a Deployment-level HTTPRoute creation assertion is genuinely missing (no `It` in `httproute_test.go` lists HTTPRoutes for a Deployment target), add one modeled on the existing statefulset HTTPRoute assertion before considering this step done; otherwise proceed.

- [ ] **Step 7: Commit**

```bash
git add test/e2e/
git commit -m "test(e2e): drop ingress coverage, keep httproute"
```

---

## Task 9: Prune the Helm chart

**Files:**
- Modify: `charts/oidc-apps/templates/clusterrole.yaml`
- Modify: `charts/oidc-apps/values.yaml`
- Modify: `charts/oidc-apps/values.schema.json`
- Modify: `charts/oidc-apps/Chart.yaml`

- [ ] **Step 1: Remove Ingress RBAC and de-gate HTTPRoute RBAC in `clusterrole.yaml`**

Delete the Ingress rule (lines 18–20):

```yaml
  - apiGroups: [ "networking.k8s.io" ]
    resources: [ "ingresses" ]
    verbs: [ "create", "get", "list", "watch", "update", "patch", "delete" ]
```

Remove the outer `httpRoutes.enabled` guard so the `httproutes` rule always renders, keeping the inner `managed` guard. Replace lines 35–44 with:

```yaml
  - apiGroups: ["gateway.networking.k8s.io"]
    resources: ["httproutes"]
    verbs: [ "create", "get", "list", "watch", "update", "patch", "delete" ]
  {{- if .Values.global.gateway.managed }}
  - apiGroups: ["gateway.networking.k8s.io"]
    resources: ["gateways"]
    verbs: [ "create", "get", "list", "watch", "update", "patch", "delete" ]
  {{- end }}
```

- [ ] **Step 2: Edit `values.yaml`**

Remove the `httpRoutes:` / `enabled:` keys from `global.gateway` (lines 130–132):

```yaml
    httpRoutes:
      enabled: false
```

Update the preceding comment so it no longer references enablement; keep the "requires Gateway API CRDs" note and make it unconditional:

```yaml
  # Gateway API configuration
  # The controller always watches Gateway API HTTPRoute resources.
  # Requires Gateway API CRDs to be installed in the cluster.
  gateway:
    # When managed is true, the controller creates and maintains a Gateway resource
    managed: false
```

Delete the entire per-target `ingress:` block (lines 227–244, from `# Target ingress entry-point` comment through `defaultPath:` under `ingress:`). Update the `httpRoute:` block comment (line 245) to drop the "requires global.gateway.httpRoutes.enabled: true" clause:

```yaml
    # Target HTTPRoute entry-point
    # Note: Each target can override global oauth2Proxy, kubeRbacProxy, labels, annotations,
    # domainName, oidcCABundle, and oidcCASecretRef configurations
    httpRoute:
```

- [ ] **Step 3: Edit `values.schema.json`**

Delete the `httpRoutes` property object under `global.gateway.properties` (lines 288–295):

```json
            "httpRoutes": {
              "type": "object",
              "properties": {
                "enabled": {
                  "type": "boolean",
                  "description": "Enable Gateway API HTTPRoute support"
                }
              }
            },
```

Delete the entire target `ingress` property definition (lines 552–596, the `"ingress": { ... }` object through its closing `}` and trailing comma before `"httpRoute"`). Keep the `httpRoute` property.

- [ ] **Step 4: Bump the chart version**

In `charts/oidc-apps/Chart.yaml`, change `version: 0.3.0` to `version: 0.4.0`. Leave `appVersion` unchanged.

- [ ] **Step 5: Lint and render**

Run: `helm lint charts/oidc-apps && helm template charts/oidc-apps > /tmp/oidc-apps-render.yaml`
Expected: lint passes; render succeeds and `/tmp/oidc-apps-render.yaml` contains the `httproutes` ClusterRole rule and no `ingresses` rule. Spot-check:

Run: `grep -c "ingresses" /tmp/oidc-apps-render.yaml`
Expected: `0`.

- [ ] **Step 6: Commit**

```bash
git add charts/oidc-apps/
git commit -m "chore(chart): drop ingress rbac/values, ungate httproute, bump to 0.4.0"
```

---

## Task 10: Update docs and examples

**Files:**
- Modify: `example/config.yaml`
- Modify: `example/kind-setup/README.md`
- Modify: `README.md`
- Create: `CHANGELOG.md`

- [ ] **Step 1: Remove the ingress example from `example/config.yaml`**

Delete the per-target `ingress:` block (lines 72–86, from `# Target ingress entry-point` through `ingressClassName:`). Ensure a `httpRoute:` example is present for the target as the migration reference; if none exists in this example file, add one modeled on `charts/oidc-apps/values.yaml`:

```yaml
    # Target HTTPRoute entry-point
    httpRoute:
      create: #false
      hostPrefix: ""
      host:
      annotations: {}
      labels: {}
      parentRefs: []
      # - name: my-gateway
      #   namespace: gateway-system
      #   sectionName: https
      defaultPath:
```

- [ ] **Step 2: Reword `example/kind-setup/README.md`**

Change the line reading "Envoy Gateway (Gateway API) for ingress" (line 5) to "Envoy Gateway (Gateway API) as the routing entry-point" so it no longer implies networking.k8s.io Ingress.

- [ ] **Step 3: Note the removal in `README.md`**

Add a short note (near the configuration/routing section) stating:

```markdown
> **Note:** As of chart v0.4.0, Kubernetes `networking.k8s.io/Ingress` is no
> longer supported. Routing is handled exclusively via the Gateway API
> (`HTTPRoute`). The Gateway API CRDs must be installed in the cluster. The
> `global.gateway.httpRoutes.enabled` flag and per-target `ingress:` config
> have been removed; migrate `ingress:` blocks to `httpRoute:`.
```

Grep the README for any remaining `ingress:` config snippet and remove/convert it:

Run: `grep -ni "ingress" README.md`
Expected after edit: only the removal note above (no live `ingress:` config examples).

- [ ] **Step 4: Create `CHANGELOG.md`**

No changelog exists today. Create `CHANGELOG.md` at the repo root:

```markdown
# Changelog

## Unreleased

### Breaking changes

- **Dropped Kubernetes Ingress support.** Routing is now handled exclusively via
  the Gateway API (`HTTPRoute`). The per-target `ingress:` configuration and the
  global `global.gateway.httpRoutes.enabled` flag have been removed. Gateway API
  HTTPRoute support is always on; the Gateway API CRDs must be installed in every
  cluster running the controller.
- Config is parsed non-strictly, so an existing `ingress:` block is **silently
  ignored** on upgrade — affected targets stop getting a routing resource with no
  error. Migrate each `ingress:` block to the equivalent `httpRoute:` block.
- Helm chart bumped to `0.4.0` to reflect the removed values keys.
```

- [ ] **Step 5: Commit**

```bash
git add example/config.yaml example/kind-setup/README.md README.md CHANGELOG.md
git commit -m "docs: drop ingress from examples and document gateway-only migration"
```

---

## Task 11: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Run the full check suite**

Run: `make check`
Expected: tidy + fmt + gci + lint all pass with no diff.

- [ ] **Step 2: Run unit tests**

Run: `make test`
Expected: PASS.

- [ ] **Step 3: Run integration tests**

Run: `make envtest`
Expected: PASS.

- [ ] **Step 4: Confirm no stray Ingress references remain**

Run: `grep -rni "networkingv1\|networking.k8s.io/v1\|IngressConf\|ShallCreateIngress\|GetIngress\|IngressName\|IsHTTPRouteEnabled\|GetHTTPRouteHost\|resolveHost\|IngressMapFuncForStatefulset" cmd/ pkg/ test/ charts/`
Expected: no matches (empty output). Any match is a missed removal — fix before finishing.

- [ ] **Step 5: Final confirmation (no commit — all work already committed per task)**

If `make check` produced formatting changes, amend the relevant task's commit or add a `chore: formatting` commit.

---

## Self-Review Notes

- **Spec coverage:** §1 config → Tasks 1–2; §2 controllers/manager → Tasks 3–4; §3 webhook → Task 5; §4 chart → Task 9; §5 tests → Tasks 7–8; §6 docs/examples → Task 10; constants → Task 6; verification → Task 11. All spec sections mapped.
- **Decisions honored:** HTTPRoute always-on (Tasks 2,4,9); managed stays opt-in (Task 2 `IsManagedGatewayEnabled`, Task 9 inner guard); single `GetHost()` (Tasks 1,5); documented hard break + silent-ignore (Task 10 CHANGELOG/README).
- **Type consistency:** `GetHost()` signature stable across Tasks 1/5; `ShallCreateHTTPRoute()`/`IsManagedGatewayEnabled()` signatures unchanged, only bodies de-gated; `fetchOidcAppsHTTPRoutes` keeps its `cfg` parameter.
- **Extra finding folded in:** `TestGetRedirectURLWithHTTPRoute` expectation changes because `GetHost()` now reads `httpRoute.Host` (Task 1 Step 3) — this was not in the spec's test list but is a direct consequence of the host merge.
