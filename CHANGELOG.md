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
