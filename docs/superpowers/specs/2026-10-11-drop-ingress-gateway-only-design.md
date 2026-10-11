# Drop Ingress support — Gateway API only

**Date:** 2026-10-11
**Status:** Approved design, ready for implementation plan

## Problem

Kubernetes `networking.k8s.io/Ingress` is deprecated in favour of the Gateway
API. The oidc-apps-controller currently supports both: it creates an `Ingress`
(always wired in, per-target `ingress.create`) **and**, optionally, a Gateway
API `HTTPRoute` (gated globally by `global.gateway.httpRoutes.enabled`). We are
dropping Ingress entirely and keeping only the Gateway API path.

## Decisions

Four decisions settle the end state (confirmed with the maintainer):

1. **HTTPRoute becomes always-on.** The global `httpRoutes.enabled` flag and its
   `IsHTTPRouteEnabled()` gate are removed. Gateway API wiring (scheme, cache,
   index, watches) is always installed. HTTPRoute creation is driven purely
   per-target by `httpRoute.create`.
2. **Managed Gateway stays opt-in.** `global.gateway.managed` continues to
   independently control whether the controller creates/reconciles its own
   Gateway resource. Its gate is simplified to no longer depend on
   `IsHTTPRouteEnabled()`.
3. **Host functions merge into one `GetHost()`.** `GetHTTPRouteHost()` is
   deleted; the single `GetHost()` reads `target.HTTPRoute`. The OAuth2 redirect
   URL and the webhook host annotation both flow through it.
4. **Documented hard break.** `IngressConf` / `Target.Ingress` and
   `httpRoutes.enabled` are removed. Config is decoded with non-strict
   `sigs.k8s.io/yaml`, so a leftover `ingress:` block is **silently ignored** —
   routes simply stop being created on upgrade. This must be documented
   (README + CHANGELOG) because there is no parse error to alert users.

## Consequence: Gateway API CRDs now required

Because Gateway API types are always installed into the scheme and watched, the
`HTTPRoute` CRD (and `Gateway` CRD when `managed` is enabled) **must be present
in every cluster** the controller runs in. Previously this was conditional on
the flag. Without the CRD, the cache informer for `HTTPRoute` fails to start.
This is called out in README and values.yaml.

## Scope of change

Approach: straight deletion mirroring the existing parallel Gateway structure.
No intermediate behaviour-change phases, no dormant/flagged dead code. The work
is split into logically-separated commits (config/API, controller+manager,
webhook, chart, tests/docs) per Conventional Commits.

### 1. Configuration & API (`pkg/configuration/`)

- **`types.go`**: remove `Target.Ingress *IngressConf` field and the entire
  `IngressConf` struct. `Target.HTTPRoute` / `HTTPRouteConf` unchanged.
- **`ingress.go`**: delete the file. Relocate the shared default-path helper
  (`isValidDefaultPath`, `validDefaultPathRe`, `maxDefaultPathLength`) into
  `gateway.go` — it is used by `GetHTTPRouteDefaultPath`. All `GetIngress*` /
  `ShallCreateIngress` getters are deleted.
- **`general.go`**: rework `GetHost()` to read `target.HTTPRoute` (HostPrefix /
  Host) instead of `target.Ingress`. `GetUpstreamTarget()` unaffected.
- **`gateway.go`**:
  - Delete `GetHTTPRouteHost()`; its one caller (`oidc-apps-httproutes.go`)
    uses `GetHost()`.
  - Remove `IsHTTPRouteEnabled()` and `isHTTPRouteEnabled()`.
  - `ShallCreateHTTPRoute()` loses its `if !isHTTPRouteEnabled()` guard; driven
    by `target.HTTPRoute.Create`.
  - `IsManagedGatewayEnabled()` simplified to
    `Global.Gateway != nil && Global.Gateway.Managed`.

### 2. Controller & manager (`pkg/controllers/`, `pkg/oidc-apps/cmd.go`)

- **`oidc-apps-ingresses.go`**: delete the entire file (all five
  builder/mutator functions). Shared helpers `fetchStrIndexIfPresent` /
  `addOptionalIndex` live elsewhere and stay.
- **`modifiers.go`**: delete `fetchOidcAppsIngress`,
  `reconcileIngressForDeployment`, `reconcileIngressForStatefulSetPod`, and
  their call sites in `reconcileDeploymentDependencies` /
  `reconcileStatefulSetDependencies`. Drop the `networkingv1` import. HTTPRoute
  reconcile calls (guarded by surviving `ShallCreateHTTPRoute()`) stay.
- **`owned.go`**: delete the Ingress fetch+delete loop in
  `deleteOwnedResources`. HTTPRoute/service/secret cleanup stays.
- **`cmd.go`**:
  - Drop `networkingv1` import; remove the `Ingress` cache entry, field index,
    `IngressMapFuncForStatefulset`, and both Ingress `.Watches(...)`.
  - De-gate HTTPRoute: the gateway scheme install, HTTPRoute cache entry,
    HTTPRoute index, and both HTTPRoute watches become unconditional (remove the
    `if httpRouteEnabled { ... }` wrappers; delete the `httpRouteEnabled` local
    and its log line).
  - Managed Gateway (`addManagedGateway`, Gateway cache entry) stays gated on
    `IsManagedGatewayEnabled()` (now `Global.Gateway.Managed`).

### 3. Webhook (`pkg/webhook/`)

- **`modifiers.go`**: delete `resolveHost` (its only job was choosing between two
  host functions). Callers use `rc.GetHost()` directly.
- **`pod-webhook.go`**: replace the two `resolveHost(p.Config, rc)` calls with
  `rc.GetHost()`. Reword the stale "creates a new ingress for each pod" comment
  to refer to the per-pod HTTPRoute/host. `oauth2.go` redirect URL already calls
  `GetHost()` — no change, follows the reworked method automatically.

### 4. Helm chart (`charts/oidc-apps/`)

- **`templates/clusterrole.yaml`**: remove the `networking.k8s.io/ingresses`
  rule (lines 18–20). De-gate HTTPRoute RBAC by removing the outer
  `{{- if ... httpRoutes.enabled }}` wrapper (lines 35, 44) so the `httproutes`
  rule always renders; keep the inner `{{- if .Values.global.gateway.managed }}`
  guard around the `gateways` rule.
- **`values.yaml`**: remove the per-target `ingress:` block; remove
  `global.gateway.httpRoutes.enabled`. Keep the rest of `global.gateway`
  (`managed`, `name`, `gatewayClassName`, listeners, infrastructure) and the
  per-target `httpRoute:` block. Update the Gateway-API-CRD comment from
  "expected pre-installed" to "required".
- **`values.schema.json`**: remove the target `ingress` property (incl.
  `ingressClassName` etc.) and the `httpRoutes.enabled` property; keep
  `httpRoute` and the gateway schema.
- **`Chart.yaml` — bump the chart `version` (required).** This change removes
  `values.yaml` keys (`target.ingress`, `global.gateway.httpRoutes.enabled`) and
  alters templates, so it is a breaking chart change. Bump `version: 0.3.0` →
  `0.4.0` (minor, signalling the removed/renamed config surface). `appVersion`
  tracks the controller image and is bumped separately at release, not here.
- Run `helm lint` + `helm template charts/oidc-apps` and confirm valid YAML.

### 5. Tests (`pkg/`, `test/e2e/`)

- **`pkg/configuration/config_test.go`**: remove the ingress-specific tests
  (`TestTargetIngressHostPrefix`, `TestTargetIngressHost`,
  `TestTargetWithoutIngressHost`, `ShallCreateIngress` assertions,
  `TestGetIngressDefaultPath`, `TestGetIngressAnnotationsAndLabels`,
  `TestGetIngressClassName`). Rewrite the `GetHost` host/host-prefix coverage
  against `HTTPRouteConf` (it now flows through `target.HTTPRoute`). Default-path
  validation is already covered by the HTTPRoute getter — drop the ingress
  duplicate after confirming parity.
- **`pkg/controllers/oidc-apps-ingresses_test.go`**: contains only
  `TestFetchStrIndexIfPresent` (a shared helper). Move it to a non-ingress file
  and delete the ingress test file.
- **envtest (`test/e2e/`)**:
  - `deployment_test.go` / `statefulset_test.go`: remove the blocks asserting an
    oidc-apps Ingress is created. Confirm HTTPRoute creation for both workload
    kinds is exercised (in `httproute_test.go` or ported); add cases if an
    ingress-only scenario is lost.
  - `suite_test.go`: remove `networkingv1.Ingress` scheme registration and the
    `IngressMapFuncForStatefulset` wiring; ensure gateway scheme + HTTPRoute
    mapfunc are registered unconditionally.
  - `setup.go`: remove `skipIngressTarget` fixtures and
    `createTargetSkipIngressStatefulSet` / `createSkipIngressStatefulSetPod`;
    repoint any dependents to httproute equivalents.
  - Gateway API CRDs already load from `test/e2e/crds/` — no fixture change.

### 6. Docs & examples (explicit update list)

Every user-facing document that mentions Ingress or the removed flag must be
updated in this change — not left for later:

- **`example/config.yaml`**: remove the per-target `ingress:` example block.
  Ensure a working `httpRoute:` example remains so users have a migration
  reference.
- **`example/kind-setup/README.md`**: reword "Envoy Gateway (Gateway API) for
  ingress" to avoid implying networking.k8s.io Ingress.
- **`README.md`**: add a short note that networking.k8s.io Ingress is no longer
  supported and Gateway API CRDs are **required**; verify no stale `ingress:`
  config snippet remains.
- **CHANGELOG**: no `CHANGELOG` file exists at the repo root today. Create one (or
  append if release notes live elsewhere) with an entry stating `target.ingress`
  and `global.gateway.httpRoutes.enabled` are removed; existing `ingress:` blocks
  are silently ignored on upgrade (non-strict decode) and users must migrate to
  `httpRoute:`. Note the Helm chart `version` bump (0.3.0 → 0.4.0) in the same
  entry.

## Constants (`pkg/constants/const.go`)

- Remove `IngressName = "oauth2-ingress"`. Keep `HTTPRouteName`,
  `ManagedGatewayName`, and all shared constants (`AnnotationHostKey`,
  `LabelKey`/`LabelValue`, `ServiceNameOauth2Service`).

## Removal-order cautions

1. `GetHost()` depends on `target.Ingress` today — rework it to `target.HTTPRoute`
   **before** deleting `IngressConf`, or the package won't compile.
2. `isValidDefaultPath` is shared with HTTPRoute — relocate it out of
   `ingress.go` before deleting that file.
3. `cmd.go` Ingress wiring must be removed as a unit (cache/index/watches/
   mapfunc) alongside de-gating HTTPRoute, so the manager still has exactly one
   routing path registered.

## Verification

- `make check` — tidy + fmt + gci + lint.
- `make test` — unit tests.
- `make envtest` — integration tests (highest-risk: suite wiring changes).
- `helm lint` + `helm template charts/oidc-apps` render valid YAML.

## Out of scope

- No migration tooling or automatic translation of `ingress:` → `httpRoute:`.
- No changes to oauth2-proxy / kube-rbac-proxy sidecar logic beyond the host
  function rename.
- No unrelated refactoring of the Gateway path.
