# SPDX-FileCopyrightText: 2026 nickytd
# SPDX-License-Identifier: Apache-2.0
CONTROLLER_NAME             := oidc-apps
WATCHER_NAME                := watcher
REGISTRY                    ?= ghcr.io/nickytd/oidc-apps
CONTROLLER_IMAGE            := $(REGISTRY)/$(CONTROLLER_NAME)
WATCHER_IMAGE               := $(REGISTRY)/kube-rbac-proxy
REPO_ROOT                   := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
VERSION                     := $(shell cat "$(REPO_ROOT)/VERSION")
EFFECTIVE_VERSION           := $(VERSION)-$(shell git rev-parse --short HEAD)
SRC_DIRS                    := $(shell go list -f '{{.Dir}}' $(REPO_ROOT)/...)
BUILD_PLATFORM              ?= $(shell go env GOOS)
BUILD_ARCH                  ?= $(shell go env GOARCH)

TOOLS_DIR                   := $(REPO_ROOT)/tools
TOOLS_MOD                   := $(TOOLS_DIR)/go.mod
GO_TOOL                     := go tool -modfile=$(TOOLS_MOD)
ENVTEST_K8S_VERSION         ?= 1.35.0

GCI_OPT                     ?= -s standard -s default -s "prefix($(shell go list -m))" --skip-generated

ifneq ($(strip $(shell git status --porcelain 2>/dev/null)),)
	EFFECTIVE_VERSION := $(EFFECTIVE_VERSION)-dirty
	GIT_TREE_STATE   := dirty
else
	GIT_TREE_STATE   := clean
endif
IMAGE_TAG                   := $(EFFECTIVE_VERSION)

# Version ldflags for k8s.io/component-base/version
VERSION_PKG                 := k8s.io/component-base/version
VERFLAG_PKG                 := $(VERSION_PKG)/verflag
GIT_COMMIT                  := $(shell git rev-parse --verify HEAD)
BUILD_DATE                  := $(shell date '+%Y-%m-%dT%H:%M:%S%z' | sed 's/\([0-9][0-9]\)$$/:&/')
VERSION_MAJOR               := $(word 1,$(subst ., ,$(VERSION:v%=%)))
VERSION_MINOR               := $(word 2,$(subst ., ,$(VERSION:v%=%)))
ld_flags                     = -w -s \
  -X $(VERSION_PKG).gitMajor=$(VERSION_MAJOR) \
  -X $(VERSION_PKG).gitMinor=$(VERSION_MINOR) \
  -X $(VERSION_PKG).gitVersion=$(EFFECTIVE_VERSION) \
  -X $(VERSION_PKG).gitTreeState=$(GIT_TREE_STATE) \
  -X $(VERSION_PKG).gitCommit=$(GIT_COMMIT) \
  -X $(VERSION_PKG).buildDate=$(BUILD_DATE) \
  -X $(VERFLAG_PKG).programName=$(1)

#########################################
# Targets                               #
#########################################
.DEFAULT_GOAL := all
all: check build test envtest

.PHONY: verify
verify: check check-go-fix test envtest sast

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################
.PHONY: docker-images
docker-images: docker-image-oidc-apps docker-image-watcher

.PHONY: docker-image-oidc-apps
docker-image-oidc-apps:
	@docker build \
		--platform linux/$(BUILD_ARCH) \
		--tag $(CONTROLLER_IMAGE):latest \
		--tag $(CONTROLLER_IMAGE):$(IMAGE_TAG) \
		-f Dockerfile.oidc-apps $(REPO_ROOT)

.PHONY: docker-image-watcher
docker-image-watcher:
	@docker build \
		--platform linux/$(BUILD_ARCH) \
		--tag $(WATCHER_IMAGE):latest \
		--tag $(WATCHER_IMAGE):$(IMAGE_TAG) \
		-f Dockerfile.watcher $(REPO_ROOT)

.PHONY: docker-push
docker-push:
	@docker push $(CONTROLLER_IMAGE):latest
	@docker push $(CONTROLLER_IMAGE):$(IMAGE_TAG)
	@docker push $(WATCHER_IMAGE):latest
	@docker push $(WATCHER_IMAGE):$(IMAGE_TAG)


#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################
.PHONY: tidy
tidy:
	@go mod tidy
	@cd $(TOOLS_DIR) && go mod tidy

.PHONY: gci
gci: tidy
	@echo "Running gci..."
	@$(GO_TOOL) gci write $(GCI_OPT) $(SRC_DIRS)

.PHONY: fmt
fmt: tidy
	@echo "Running $@..."
	@$(GO_TOOL) golangci-lint fmt \
    	--config=$(REPO_ROOT)/.golangci.yaml \
    	$(SRC_DIRS)

.PHONY: check-go-fix
check-go-fix: tidy
	@echo "Running go fix..."
	@go fix $(SRC_DIRS)/...
	@if [ -n "$$(git status --porcelain $(SRC_DIRS))" ]; then \
		echo "Error: go fix produced changes. Please run 'go fix ./...' and commit the changes."; \
		git --no-pager diff; \
		exit 1; \
	fi

.PHONY: check
check: tidy fmt gci lint

.PHONY: lint
lint: tidy
	@echo "Running $@..."
	@$(GO_TOOL) golangci-lint run \
	 	--config=$(REPO_ROOT)/.golangci.yaml \
		$(SRC_DIRS)

.PHONY: build
build: build-oidc-apps build-watcher

.PHONY: build-oidc-apps
build-oidc-apps: tidy
	@CGO_ENABLED=0 go build -ldflags="$(call ld_flags,oidc-apps)" \
	  	-o $(REPO_ROOT)/bin/oidc-apps $(REPO_ROOT)/cmd/oidc-apps

.PHONY: build-watcher
build-watcher: tidy
	@CGO_ENABLED=0 go build -ldflags="$(call ld_flags,watcher)" \
		-o $(REPO_ROOT)/bin/watcher $(REPO_ROOT)/cmd/watcher

.PHONY: clean
clean:
	@echo "Running $@..."
	@rm -f $(REPO_ROOT)/bin/oidc-apps
	@rm -f $(REPO_ROOT)/bin/watcher
	@$(GO_TOOL) setup-envtest cleanup --bin-dir=$(TOOLS_DIR)


.PHONY: test
test: tidy
	@go generate $(SRC_DIRS)
	@$(GO_TOOL) gotestsum --format-hide-empty-pkg $(REPO_ROOT)/cmd/... $(REPO_ROOT)/pkg/...

.PHONY: envtest
envtest: tidy
	@KUBEBUILDER_ASSETS=$(shell \
		$(GO_TOOL) setup-envtest \
		use $(ENVTEST_K8S_VERSION) \
		--bin-dir=$(TOOLS_DIR) \
		-p path 2>/dev/null || true) \
		$(GO_TOOL) gotestsum \
			--format-hide-empty-pkg \
			$(REPO_ROOT)/test/... \
			--ginkgo.v \
			-timeout 10m

.PHONY: add-license-headers
add-license-headers: tidy
	@$(GO_TOOL) addlicense \
		-c "nickytd" \
		-l apache \
		-s=only \
		-y "$$(date +%Y)" \
		-ignore "$(REPO_ROOT)/.git/**" \
		-ignore "**/*.md" \
		-ignore "**/*.html" \
		-ignore "**/*.yaml" \
		-ignore "**/Dockerfile*" \
		$(REPO_ROOT)

.PHONY: govulncheck
govulncheck: tidy
	@$(GO_TOOL) govulncheck $(REPO_ROOT)/...

.PHONY: sast
sast: tidy
	@$(GO_TOOL) gosec -exclude-generated -exclude-dir=hack ./...
