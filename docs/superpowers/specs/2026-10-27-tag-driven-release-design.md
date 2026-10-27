# Redesign release: tag-driven, modeled on fluent-bit-output-go

**Date:** 2026-10-27
**Status:** Approved

## Background / problem

The Release workflow run for commit `86eac58` failed in the `manifest` job:

```
ghcr.io/nickytd/oidc-apps/oidc-apps:v0.5.2-amd64 is a manifest list
##[error]Process completed with exit code 1.
```

Two root causes, both addressed by this redesign:

1. **Assembly bug.** `build` ran `docker build --platform <arch>` + `docker push`
   per arch. Current BuildKit defaults attach a provenance attestation, so each
   per-arch push is an OCI **index**, not a bare image. The `manifest` job then
   called `docker manifest create`, which refuses an index-typed child → exit 1.
   `manifest` failing skipped `release`, so no tag and no GitHub Release were
   created.
2. **Fragile trigger machinery.** The release is triggered by pushing changes to
   a `VERSION` file on `main`, where each release commit bumps `VERSION` to the
   *next* number — so a "release v0.5.1" commit actually builds v0.5.2. The
   `prepare` job also sed-edits `charts/oidc-apps/Chart.yaml` `appVersion` and
   `git push`es it back mid-release, which previously caused a non-fast-forward
   race (the reason the `concurrency` guard was added).

The sibling project `nickytd/fluent-bit-output-go` already solved both: releases
are triggered by pushing a semver tag, images are built with
`docker/build-push-action` (`provenance: false`, `sbom: false`), and the
multi-arch index is assembled with `docker buildx imagetools create`. This
redesign ports that model to oidc-apps.

## Design

### 1. Trigger: tag push

```yaml
on:
  push:
    tags: ['v*.*.*']
  workflow_dispatch:
    inputs:
      tag:
        description: 'Tag to release (e.g. v0.5.2)'
        required: true
```

`prepare` resolves the version from `github.ref_name` (tag push) or
`inputs.tag` (manual dispatch). **The tag is the single source of truth.**
No VERSION file read, no tag creation inside the workflow (the tag already
exists — it is the trigger).

### 2. Delete the `VERSION` file

- Remove `/VERSION`.
- Makefile: make `VERSION` an **overridable** variable that falls back to git
  when not supplied:
  ```makefile
  VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo v0.0.0-unknown)
  ```
  (`?=` so `make VERSION=v0.5.2 build` overrides it.) Local `make build` then
  bakes a meaningful version into the `k8s.io/component-base/version` ldflags
  with no VERSION file present.
- **Docker build caveat (the delicate part):** `make build` runs *inside* the
  Docker builder stage via `COPY . . && RUN make build`. There is no `.git`
  directory in the build context, so the `git describe` / `git rev-parse` /
  `git status` shell-outs in the Makefile return empty inside Docker. Two
  consequences the implementation must handle:
  1. The Dockerfiles must pass the version in explicitly:
     `ARG VERSION` → `RUN make VERSION=${VERSION} build`. The make-variable
     override bypasses the (failed) git lookup for the version string.
  2. The remaining git-derived ldflags fields (`gitCommit`, `GIT_TREE_STATE`,
     `EFFECTIVE_VERSION`'s short-sha suffix) must degrade gracefully when the
     git commands fail — guard them so an empty `git rev-parse` does not
     produce a broken ldflag. `gitCommit` can fall back to the `REVISION`
     build-arg (`github.sha`) passed alongside `VERSION`, mirroring
     fluent-bit-output-go's `REVISION` build-arg.

This is the only net-new work beyond porting the reference, because oidc-apps
bakes the version into Go ldflags and fluent-bit-output-go does not.

### 3. `build` job — native runners + build-push-action

Keep the native-runner matrix (amd64 on `ubuntu-latest`, arm64 on
`ubuntu-24.04-arm`) and the per-arch tag scheme. Replace raw
`docker build`/`docker push` with `docker/build-push-action@v7`:

```yaml
      - uses: docker/setup-buildx-action@v4
      - uses: docker/build-push-action@v7
        with:
          context: .
          file: Dockerfile.oidc-apps        # and Dockerfile.watcher
          platforms: ${{ matrix.platform }}
          push: true
          tags: ghcr.io/nickytd/oidc-apps/oidc-apps:${{ needs.prepare.outputs.version }}-${{ matrix.suffix }}
          build-args: |
            VERSION=${{ needs.prepare.outputs.version }}
            REVISION=${{ github.sha }}
          provenance: false
          sbom: false
```

`provenance: false` + `sbom: false` make each per-arch push a plain image
manifest — fixing the `is a manifest list` failure at its source. Both images
(`oidc-apps` and `kube-rbac-proxy`) are built this way.

### 4. `manifest` job — imagetools

```yaml
      - uses: docker/setup-buildx-action@v4
      - run: |
          VERSION="${{ needs.prepare.outputs.version }}"
          for IMG in oidc-apps kube-rbac-proxy; do
            REPO="ghcr.io/nickytd/oidc-apps/${IMG}"
            docker buildx imagetools create \
              --tag "${REPO}:${VERSION}" \
              --tag "${REPO}:latest" \
              "${REPO}:${VERSION}-amd64" \
              "${REPO}:${VERSION}-arm64"
          done
```

`imagetools create` is the correct assembler (and handles index-typed children
if attestations ever reappear). One invocation emits both `:VERSION` and
`:latest` and pushes immediately.

### 5. Helm chart — versions injected at package time

Remove the sed-edit-and-commit-back of `Chart.yaml` from `prepare`. Package
with versions injected, no git write-back:

```yaml
      - run: |
          VERSION="${{ needs.prepare.outputs.version }}"
          helm package charts/oidc-apps \
            --version "${VERSION#v}" \
            --app-version "${VERSION}"
          helm push oidc-apps-*.tgz oci://ghcr.io/nickytd/oidc-apps/charts
```

`Chart.yaml` keeps **placeholder** values, explicitly commented:

```yaml
version: 0.0.0          # placeholder — real version injected at package time (--version)
appVersion: "0.0.0"     # placeholder — real version injected at package time (--app-version)
```

This eliminates the mid-release `git push` and the non-fast-forward race the
`concurrency` guard worked around.

### 6. `release` job

Drop the `git pull` and `git tag` creation steps. Just publish:

```yaml
      - uses: actions/checkout@v7
        with: { fetch-depth: 0 }
      - uses: softprops/action-gh-release@v3
        with:
          tag_name: ${{ needs.prepare.outputs.version }}
          name: ${{ needs.prepare.outputs.version }}
          generate_release_notes: true
```

### Out of scope

- `.github/workflows/build.yml` is left untouched — it already serves as the CI
  half (lint/test/envtest/build on push and PR), matching the reference's
  CI/Release separation.
- `scorecard.yml` unchanged.

## Files touched

- `.github/workflows/release.yml` — rewrite (tag trigger, build-push-action,
  imagetools, package-time chart versions, no tag/appVersion commit-back).
- `Makefile` — derive version from `git describe` instead of `cat VERSION`.
- `Dockerfile.oidc-apps`, `Dockerfile.watcher` — accept `ARG VERSION` and pass
  it to the build.
- `charts/oidc-apps/Chart.yaml` — placeholder `version`/`appVersion` + comments.
- `VERSION` — deleted.

## Verification

- **Static:** `actionlint` on the workflow (if available); `make build` locally
  to confirm the Makefile version-derivation change bakes a sane version with no
  VERSION file present; `helm package --version … --app-version …` dry run to
  confirm the chart packages cleanly from placeholders.
- **Live:** CI-only; cannot be fully exercised locally. After merge, push tag
  `v0.5.2` (`git tag v0.5.2 && git push origin v0.5.2`) to run the full release
  end-to-end. This also performs the recovery — v0.5.2 was never published.

## Recovery decision

v0.5.1 was never published and is superseded. First release under the new
workflow is **v0.5.2**, created by pushing the `v0.5.2` tag after the fix
merges. No history rewriting.

## New release procedure

```
git tag v0.5.2 && git push origin v0.5.2
```

That is the entire release action.
