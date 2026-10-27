# Tag-Driven Release Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the VERSION-file-triggered release workflow with a tag-driven one (`git push origin vX.Y.Z` → release), fixing the multi-arch manifest failure that left v0.5.1 unpublished.

**Architecture:** Port the proven model from `nickytd/fluent-bit-output-go`: releases trigger on semver tag push; per-arch images build on native runners via `docker/build-push-action` (`provenance: false`, `sbom: false`) so each push is a plain image; the multi-arch index is assembled with `docker buildx imagetools create`. The git tag is the single source of truth — the `VERSION` file is deleted and the Makefile derives version from `git describe`, overridable via `make VERSION=…` for in-Docker builds.

**Tech Stack:** GitHub Actions, Docker buildx, Helm (OCI push), Go (ldflags via `k8s.io/component-base/version`), Make.

## Global Constraints

- Go module: `github.com/nickytd/oidc-apps`; Go 1.27.
- Registry: `ghcr.io/nickytd/oidc-apps/` — images `oidc-apps` and `kube-rbac-proxy`; chart at `oci://ghcr.io/nickytd/oidc-apps/charts`.
- Native-runner matrix preserved: amd64 on `ubuntu-latest`, arm64 on `ubuntu-24.04-arm`. No QEMU.
- Per-arch tag scheme preserved: `:VERSION-amd64`, `:VERSION-arm64`; index tags `:VERSION` and `:latest`.
- Conventional Commits for every commit; separate logical changes into separate commits.
- License headers on source files (`# SPDX-FileCopyrightText: 2026 nickytd` / `# SPDX-License-Identifier: Apache-2.0`) — the workflow and Dockerfiles already carry them; preserve on edit.
- `build.yml` and `scorecard.yml` are out of scope — do not modify.
- First release under the new workflow is `v0.5.2` (recovers the stuck release); no history rewriting.

---

### Task 1: Make the Makefile version overridable and git-fallback safe

**Files:**
- Modify: `Makefile:9-10`, `Makefile:25-31`, `Makefile:36`

**Interfaces:**
- Produces: `make VERSION=<v> build` and `make VERSION=<v> build-watcher` bake `<v>` into the `k8s.io/component-base/version.gitVersion` ldflag. With no `VERSION=` override and no `.git`, the build still succeeds with a sane fallback (`v0.0.0-unknown`). A new overridable `REVISION ?=` make variable feeds `gitCommit`.

- [ ] **Step 1: Read current version block**

Confirm `Makefile:9` is `VERSION := $(shell cat "$(REPO_ROOT)/VERSION")` and `Makefile:10` is `EFFECTIVE_VERSION := $(VERSION)-$(shell git rev-parse --short HEAD)`.

- [ ] **Step 2: Replace the VERSION source with git-describe fallback, overridable**

Replace `Makefile:9-10`:

```makefile
# Version source: overridable (make VERSION=v1.2.3) for in-Docker builds that
# have no .git; otherwise derived from git. Falls back when git is unavailable.
VERSION                     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo v0.0.0-unknown)
GIT_SHORT                   := $(shell git rev-parse --short HEAD 2>/dev/null)
EFFECTIVE_VERSION           := $(if $(GIT_SHORT),$(VERSION)-$(GIT_SHORT),$(VERSION))
```

- [ ] **Step 3: Make the dirty/tree-state block git-absence safe**

Replace `Makefile:25-31` (the `ifneq ... EFFECTIVE_VERSION := ...-dirty ... GIT_TREE_STATE` block). The `git status --porcelain` already returns empty under `2>/dev/null` when `.git` is absent, so the existing `ifneq` naturally takes the else branch (`clean`). Only change: guard against double-appending `-dirty` when `git describe --dirty` already added it:

```makefile
ifneq ($(strip $(shell git status --porcelain 2>/dev/null)),)
	GIT_TREE_STATE   := dirty
else
	GIT_TREE_STATE   := clean
endif
IMAGE_TAG                   := $(EFFECTIVE_VERSION)
```

(Removes the `EFFECTIVE_VERSION := $(EFFECTIVE_VERSION)-dirty` line — `git describe --dirty` already marks dirtiness, and the in-Docker path has no git to mark it.)

- [ ] **Step 4: Make GIT_COMMIT overridable via REVISION**

Replace `Makefile:36` (`GIT_COMMIT := $(shell git rev-parse --verify HEAD)`):

```makefile
REVISION                    ?= $(shell git rev-parse --verify HEAD 2>/dev/null)
GIT_COMMIT                  := $(REVISION)
```

- [ ] **Step 5: Verify local build still bakes a version (with git present)**

Run: `make build && ./bin/oidc-apps --version 2>&1 | head -5`
Expected: prints a version string containing the `git describe` output (e.g. `v0.5.1-...` or the current tag), no `cat: VERSION: No such file` error yet (file still exists at this task — that's fine, `?=` means the git default is only used when VERSION is unset, and VERSION is not set in env). Build exits 0.

Note: if `--version` is not a supported flag, substitute `strings bin/oidc-apps | grep -m1 gitVersion` or skip output inspection and just confirm `make build` exits 0.

- [ ] **Step 6: Verify override path works (simulates in-Docker build)**

Run: `make VERSION=v9.9.9 REVISION=deadbeef build && echo OK`
Expected: exits 0, prints `OK`. (Confirms the build-arg path the Dockerfile will use.)

- [ ] **Step 7: Commit**

```bash
git add Makefile
git commit -m "build: derive version from git, overridable via make VERSION"
```

---

### Task 2: Pass VERSION/REVISION build-args through the Dockerfiles

**Files:**
- Modify: `Dockerfile.oidc-apps:1-12`
- Modify: `Dockerfile.watcher:4-9`

**Interfaces:**
- Consumes: `make VERSION=… REVISION=… build` / `build-watcher` from Task 1.
- Produces: `docker build --build-arg VERSION=<v> --build-arg REVISION=<sha>` bakes `<v>` into the binary. Without the args, build still succeeds (Makefile fallback), though version will be `v0.0.0-unknown` since `.git` is absent in the build context.

- [ ] **Step 1: Add build args to Dockerfile.oidc-apps builder stage**

In `Dockerfile.oidc-apps`, change the builder stage. Current:

```dockerfile
# Stage 1: Build the Go app
FROM golang:1.27 AS builder

# Set up the working directory
WORKDIR /src

# Copy the source code into the container
COPY . .
RUN go mod download && cd tools && go mod download

# Build the application
RUN make build
```

Replace with:

```dockerfile
# Stage 1: Build the Go app
FROM golang:1.27 AS builder

# Version info injected by CI (no .git in the build context).
ARG VERSION=v0.0.0-unknown
ARG REVISION=

# Set up the working directory
WORKDIR /src

# Copy the source code into the container
COPY . .
RUN go mod download && cd tools && go mod download

# Build the application
RUN make VERSION=${VERSION} REVISION=${REVISION} build
```

- [ ] **Step 2: Add build args to Dockerfile.watcher builder stage**

In `Dockerfile.watcher`, current builder stage:

```dockerfile
FROM golang:1.27 AS builder

WORKDIR /src
COPY . .
RUN go mod download && cd tools && go mod download
RUN make build-watcher
```

Replace with:

```dockerfile
FROM golang:1.27 AS builder

# Version info injected by CI (no .git in the build context).
ARG VERSION=v0.0.0-unknown
ARG REVISION=

WORKDIR /src
COPY . .
RUN go mod download && cd tools && go mod download
RUN make VERSION=${VERSION} REVISION=${REVISION} build-watcher
```

- [ ] **Step 3: Verify the controller image builds with build-args**

Run:
```bash
docker build --build-arg VERSION=v9.9.9 --build-arg REVISION=deadbeef \
  -t oidc-apps-verify:local -f Dockerfile.oidc-apps .
```
Expected: build completes, exits 0. (This confirms `make VERSION=… REVISION=… build` works inside the builder stage with no `.git`.)

- [ ] **Step 4: Verify the watcher image builds with build-args**

Run:
```bash
docker build --build-arg VERSION=v9.9.9 --build-arg REVISION=deadbeef \
  -t kube-rbac-proxy-verify:local -f Dockerfile.watcher .
```
Expected: build completes, exits 0.

- [ ] **Step 5: Commit**

```bash
git add Dockerfile.oidc-apps Dockerfile.watcher
git commit -m "build: accept VERSION/REVISION build-args in Dockerfiles"
```

---

### Task 3: Make Chart.yaml version/appVersion explicit placeholders

**Files:**
- Modify: `charts/oidc-apps/Chart.yaml:5-6`

**Interfaces:**
- Produces: `charts/oidc-apps/Chart.yaml` with placeholder `version`/`appVersion` that the release workflow overrides at package time via `helm package --version … --app-version …`.

- [ ] **Step 1: Replace version/appVersion with commented placeholders**

In `charts/oidc-apps/Chart.yaml`, replace lines 5-6:

```yaml
version: 0.4.0
appVersion: "v0.5.0"
```

with:

```yaml
# Placeholders — real values injected at release time by
# `helm package --version <tag-without-v> --app-version <tag>`.
version: 0.0.0
appVersion: "0.0.0"
```

- [ ] **Step 2: Verify the chart still lints/templates with placeholders**

Run: `helm lint charts/oidc-apps && helm template charts/oidc-apps >/dev/null && echo OK`
Expected: `1 chart(s) linted, 0 chart(s) failed`, then `OK`. (Placeholders are valid semver, so lint passes.)

- [ ] **Step 3: Verify package-time override produces correct metadata**

Run:
```bash
helm package charts/oidc-apps --version 0.5.2 --app-version v0.5.2 -d /tmp/chart-verify \
  && tar -xzOf /tmp/chart-verify/oidc-apps-0.5.2.tgz oidc-apps/Chart.yaml | grep -E '^(version|appVersion):'
```
Expected:
```
version: 0.5.2
appVersion: v0.5.2
```
(Confirms the workflow's package-time injection overrides the placeholders. Clean up: `rm -rf /tmp/chart-verify`.)

- [ ] **Step 4: Commit**

```bash
git add charts/oidc-apps/Chart.yaml
git commit -m "chore(chart): make version/appVersion placeholders set at package time"
```

---

### Task 4: Rewrite the release workflow to be tag-driven

**Files:**
- Modify: `.github/workflows/release.yml` (full rewrite of triggers + all four jobs)

**Interfaces:**
- Consumes: Makefile `VERSION`/`REVISION` overrides (Task 1), Dockerfile build-args (Task 2), Chart placeholders (Task 3).
- Produces: a workflow that, on `git push origin vX.Y.Z`, builds per-arch images, assembles `:VERSION`/`:latest` indices for both images, pushes the Helm chart with injected versions, and creates a GitHub Release.

- [ ] **Step 1: Replace the entire release.yml**

Overwrite `.github/workflows/release.yml` with:

```yaml
# SPDX-FileCopyrightText: 2026 nickytd
# SPDX-License-Identifier: Apache-2.0

name: Release

on:
  push:
    tags:
      - 'v*.*.*'
  workflow_dispatch:
    inputs:
      tag:
        description: 'Tag to release (e.g. v0.5.2)'
        required: true

# Serialize releases on the same ref; never cancel an in-flight release.
concurrency:
  group: release-${{ github.ref }}
  cancel-in-progress: false

jobs:
  prepare:
    name: Resolve version
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    outputs:
      version: ${{ steps.version.outputs.version }}
    steps:
      - name: Resolve version from tag or input
        id: version
        run: |
          if [[ "${{ github.event_name }}" == "workflow_dispatch" ]]; then
            VERSION="${{ github.event.inputs.tag }}"
          else
            VERSION="${{ github.ref_name }}"
          fi
          echo "version=${VERSION}" >> "$GITHUB_OUTPUT"

      - name: Checkout
        uses: actions/checkout@v7
        with:
          fetch-depth: 0

      - name: Login to GHCR
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Package and push Helm chart
        run: |
          VERSION="${{ steps.version.outputs.version }}"
          helm package charts/oidc-apps \
            --version "${VERSION#v}" \
            --app-version "${VERSION}"
          helm push oidc-apps-*.tgz oci://ghcr.io/nickytd/oidc-apps/charts

  build:
    name: Build image (${{ matrix.suffix }})
    needs: prepare
    runs-on: ${{ matrix.runner }}
    permissions:
      contents: read
      packages: write
    strategy:
      fail-fast: false
      matrix:
        include:
          - runner: ubuntu-latest
            platform: linux/amd64
            suffix: amd64
          - runner: ubuntu-24.04-arm
            platform: linux/arm64
            suffix: arm64
    steps:
      - name: Checkout
        uses: actions/checkout@v7
        with:
          fetch-depth: 0

      - name: Login to GHCR
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Set up Buildx
        uses: docker/setup-buildx-action@v4

      - name: Build and push oidc-apps (${{ matrix.suffix }})
        uses: docker/build-push-action@v7
        with:
          context: .
          file: Dockerfile.oidc-apps
          platforms: ${{ matrix.platform }}
          push: true
          tags: ghcr.io/nickytd/oidc-apps/oidc-apps:${{ needs.prepare.outputs.version }}-${{ matrix.suffix }}
          build-args: |
            VERSION=${{ needs.prepare.outputs.version }}
            REVISION=${{ github.sha }}
          provenance: false
          sbom: false

      - name: Build and push kube-rbac-proxy (${{ matrix.suffix }})
        uses: docker/build-push-action@v7
        with:
          context: .
          file: Dockerfile.watcher
          platforms: ${{ matrix.platform }}
          push: true
          tags: ghcr.io/nickytd/oidc-apps/kube-rbac-proxy:${{ needs.prepare.outputs.version }}-${{ matrix.suffix }}
          build-args: |
            VERSION=${{ needs.prepare.outputs.version }}
            REVISION=${{ github.sha }}
          provenance: false
          sbom: false

  manifest:
    name: Publish multi-arch manifests
    needs: [prepare, build]
    runs-on: ubuntu-latest
    permissions:
      packages: write
    steps:
      - name: Login to GHCR
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Set up Buildx
        uses: docker/setup-buildx-action@v4

      - name: Create and push multi-arch manifests
        run: |
          VERSION="${{ needs.prepare.outputs.version }}"
          for IMG in oidc-apps kube-rbac-proxy; do
            REPO="ghcr.io/nickytd/oidc-apps/${IMG}"
            docker buildx imagetools create \
              --tag "${REPO}:${VERSION}" \
              --tag "${REPO}:latest" \
              "${REPO}:${VERSION}-amd64" \
              "${REPO}:${VERSION}-arm64"
          done

  release:
    name: Create GitHub Release
    needs: [prepare, manifest]
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - name: Checkout
        uses: actions/checkout@v7
        with:
          fetch-depth: 0

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v3
        with:
          tag_name: ${{ needs.prepare.outputs.version }}
          name: ${{ needs.prepare.outputs.version }}
          generate_release_notes: true
```

- [ ] **Step 2: Validate YAML syntax**

Run: `python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/release.yml')); print('yaml OK')"`
Expected: `yaml OK`

(If `actionlint` is available via `brew install actionlint` or `go tool`, also run `actionlint .github/workflows/release.yml` and expect no errors. It is not currently installed — YAML validation is the required check.)

- [ ] **Step 3: Verify no lingering VERSION-file or appVersion-commit logic**

Run: `grep -nE 'cat VERSION|appVersion|git tag|git push|paths:' .github/workflows/release.yml || echo "clean"`
Expected: `clean` (none of the old fragile machinery remains).

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: trigger release on tag push, fix multi-arch manifest assembly"
```

---

### Task 5: Delete the VERSION file

**Files:**
- Delete: `VERSION`

**Interfaces:**
- Consumes: Task 1 (Makefile no longer reads the file), Task 4 (workflow no longer reads the file).

- [ ] **Step 1: Confirm nothing still references the VERSION file**

Run: `grep -rnE 'cat .*VERSION|/VERSION|REPO_ROOT./VERSION' Makefile .github/ Dockerfile.* 2>/dev/null || echo "no references"`
Expected: `no references`. (If any appear, fix before deleting.)

- [ ] **Step 2: Delete the file**

Run: `git rm VERSION`
Expected: `rm 'VERSION'`

- [ ] **Step 3: Verify a clean build still works with no VERSION file**

Run: `make build && echo BUILD_OK`
Expected: exits 0, prints `BUILD_OK`. The Makefile `git describe` fallback supplies the version.

- [ ] **Step 4: Commit**

```bash
git commit -m "chore: remove VERSION file; git tag is the source of truth"
```

---

### Task 6: Document the new release procedure

**Files:**
- Modify: `.claude/CLAUDE.md` (the "Commands" or a new "Releasing" section)
- Modify: `README.md` if it documents releasing (check first)

**Interfaces:**
- Consumes: the finished workflow (Tasks 1-5).

- [ ] **Step 1: Check whether README documents releasing**

Run: `grep -niE 'release|VERSION file|appVersion' README.md | head -20 || echo "no README release docs"`
Expected: either a list of lines to update, or `no README release docs`.

- [ ] **Step 2: Add a Releasing section to `.claude/CLAUDE.md`**

Add after the "## Commands" section:

```markdown
## Releasing

Releases are triggered by pushing a semver tag — the tag is the single source
of truth (there is no VERSION file):

```bash
git tag v0.5.2
git push origin v0.5.2
```

This runs `.github/workflows/release.yml`: builds per-arch images on native
runners, assembles `:v0.5.2` and `:latest` multi-arch indices for both
`oidc-apps` and `kube-rbac-proxy`, pushes the Helm chart to
`oci://ghcr.io/nickytd/oidc-apps/charts` with `--version 0.5.2 --app-version
v0.5.2`, and creates the GitHub Release. A failed run can be re-driven via the
workflow's `workflow_dispatch` input (tag name).
```

- [ ] **Step 3: Update README if Step 1 found release docs**

If Step 1 returned lines, update them to describe tag-driven releasing (mirror the CLAUDE.md wording) and remove any mention of editing the VERSION file or the appVersion commit-back. If Step 1 returned `no README release docs`, skip this step.

- [ ] **Step 4: Verify docs no longer describe the old flow**

Run: `grep -rniE 'edit.*VERSION file|bump.*VERSION file|commit.*appVersion' .claude/CLAUDE.md README.md 2>/dev/null || echo "docs clean"`
Expected: `docs clean`

- [ ] **Step 5: Commit**

```bash
git add .claude/CLAUDE.md README.md
git commit -m "docs: document tag-driven release procedure"
```

---

## Post-implementation: cutting the recovery release

Not a code task — the operator action that recovers the stuck release and
exercises the new workflow end-to-end. After all tasks merge to `main`:

```bash
git checkout main && git pull
git tag v0.5.2
git push origin v0.5.2
```

Then watch: `gh run watch $(gh run list --workflow=Release --limit 1 --json databaseId -q '.[0].databaseId')`

Expected: all four jobs (`prepare`, `build` ×2, `manifest`, `release`) succeed;
`v0.5.2` appears in `gh release list`; `docker buildx imagetools inspect
ghcr.io/nickytd/oidc-apps/oidc-apps:v0.5.2` shows both `linux/amd64` and
`linux/arm64`.

---

## Self-Review Notes

- **Spec coverage:** tag trigger (T4), VERSION deletion + Makefile git-derive (T1, T5), Dockerfile build-args + graceful git degradation (T2, T1), build-push-action with `provenance/sbom: false` (T4), imagetools assembly (T4), package-time chart versions + placeholders (T3, T4), dropped tag-creation/appVersion-commit-back (T4), `build.yml`/`scorecard.yml` untouched (Global Constraints), v0.5.2 recovery (post-impl section). All spec sections mapped.
- **Type/name consistency:** make variables `VERSION`/`REVISION` consistent across T1→T2→T4; image repos and tag scheme consistent across T4; chart flags `--version`/`--app-version` consistent across T3→T4.
- **Placeholder scan:** no TBD/TODO; every code step shows full content; verification steps use tools confirmed present (`helm`, `docker`, `python3`), with `actionlint` noted as optional-not-installed.
