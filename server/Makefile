SHELL := /bin/bash
APP ?= velora
MODULE ?= github.com/sevoniva-labs/velora/server
GOPROXY ?= https://goproxy.cn
GOSUMDB ?= sum.golang.org https://goproxy.cn/sumdb/sum.golang.org
NPM_REGISTRY ?= https://registry.npmmirror.com
PNPM = corepack pnpm
REPO_ROOT := $(abspath $(CURDIR)/..)
WEB_DIR := $(REPO_ROOT)/web
GO_ENV = GOPROXY=$(GOPROXY) GOSUMDB='$(GOSUMDB)'
TOOL_RUN = $(GO_ENV) go run -modfile=tools/go.mod
PROTO_TOOLS = .tools/bin/buf .tools/bin/protoc-gen-go .tools/bin/protoc-gen-go-grpc .tools/bin/protoc-gen-go-http .tools/bin/protoc-gen-openapi

.PHONY: help run worker migrate storage-check pitr-check pitr-check-test test fmt tidy web-install web-api-generate web-api-check web-dev web-build web-budget web-e2e-install-cn build check contract module-boundaries proto-tools proto-lint proto-generate proto-breaking proto-check offline-check disaster-check disaster-check-certified docker-build compose-up compose-down init ai-governance apisix-policy ci-policy ci-go ci-web ci-web-e2e ci-deploy security-tools supply-chain-evidence release-evidence verify

help:
	@echo "Velora"
	@echo "  make run           Run API server"
	@echo "  make worker        Run reliable-message/background worker"
	@echo "  make migrate       Run one-shot database migrations"
	@echo "  make storage-check Read-only object-store ping and capability report"
	@echo "  make pitr-check    Read-only PostgreSQL PITR prerequisite check"
	@echo "  make web-dev       Run frontend"
	@echo "  make web-budget    Enforce production frontend bundle budgets"
	@echo "  make web-e2e-install-cn  Install validated Linux ARM64 Chromium from npmmirror"
	@echo "  make ci-web-e2e    Run frontend gates and production browser E2E"
	@echo "  make ai-governance Validate Claude/Codex/Cursor policy and Skill sync"
	@echo "  make apisix-policy Validate optional APISIX production integration"
	@echo "  make test          Run Go tests"
	@echo "  make contract      Check API error-code contract"
	@echo "  make check         Format, vet, test, contract, frontend lint/build"
	@echo "  make verify        Run the complete required CI verification gate"
	@echo "  make release-evidence  Scan, sign, and verify an internal digest image"
	@echo "  make disaster-check  Validate a dated disaster evidence report"
	@echo "  make proto-check     Lint and regenerate the Buf API contracts"
	@echo "  make compose-up    Start minimal compose stack"
	@echo "  make init APP=x MODULE=example.com/x  Rename starter"

run:
	VELORA_CONFIG=configs/minimal.yaml go run ./cmd/server

worker:
	VELORA_CONFIG=configs/minimal.yaml go run ./cmd/worker

migrate:
	VELORA_CONFIG=configs/minimal.yaml go run ./cmd/migrate

storage-check:
	go run ./cmd/storage-check

pitr-check:
	bash ../scripts/check-pitr-config.sh

pitr-check-test:
	bash ../scripts/check-pitr-config.test.sh

test:
	go test ./...

fmt:
	gofmt -w $$(find cmd internal -name '*.go')

tidy:
	go mod tidy

web-install:
	cd $(WEB_DIR) && PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 $(PNPM) install --frozen-lockfile --registry=$(NPM_REGISTRY)

web-api-generate:
	@echo "Web API client generation is not used by the Velora standalone web app"

web-api-check:
	@echo "Web API generated-client check skipped: standalone web uses src/api/api.ts"

web-dev:
	cd $(WEB_DIR) && $(PNPM) dev

web-build:
	cd $(WEB_DIR) && $(PNPM) build

web-budget:
	node scripts/check-web-bundle-budget.mjs

web-e2e-install-cn:
	$(PNPM) --filter @forge/e2e e2e:install:cn

contract:
	python3 scripts/check-error-codes.py
	python3 scripts/check-contract-coverage.py
	python3 scripts/check-openapi-security.py

module-boundaries:
	bash scripts/check-module-boundaries.sh

proto-tools: $(PROTO_TOOLS)

.tools/bin/buf: tools/go.mod tools/go.sum
	mkdir -p .tools/bin
	$(GO_ENV) go build -modfile=tools/go.mod -o .tools/bin/buf github.com/bufbuild/buf/cmd/buf

.tools/bin/protoc-gen-go: tools/go.mod tools/go.sum
	mkdir -p .tools/bin
	$(GO_ENV) go build -modfile=tools/go.mod -o .tools/bin/protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go

.tools/bin/protoc-gen-go-grpc: tools/go.mod tools/go.sum
	mkdir -p .tools/bin
	$(GO_ENV) go build -modfile=tools/go.mod -o .tools/bin/protoc-gen-go-grpc google.golang.org/grpc/cmd/protoc-gen-go-grpc

.tools/bin/protoc-gen-go-http: tools/go.mod tools/go.sum
	mkdir -p .tools/bin
	$(GO_ENV) go build -modfile=tools/go.mod -o .tools/bin/protoc-gen-go-http github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2

.tools/bin/protoc-gen-openapi: tools/go.mod tools/go.sum
	mkdir -p .tools/bin
	$(GO_ENV) go build -modfile=tools/go.mod -o .tools/bin/protoc-gen-openapi github.com/google/gnostic/cmd/protoc-gen-openapi

proto-lint: proto-tools
	.tools/bin/buf lint

proto-generate: proto-tools
	.tools/bin/buf generate --path api/proto/forge

proto-breaking: proto-tools
	.tools/bin/buf breaking --against api/buf/baseline.binpb

proto-check: proto-lint proto-breaking
	bash scripts/check-generated-proto.sh

build: web-build
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$${VERSION:-dev}" -o bin/velora ./cmd/server
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$${VERSION:-dev}" -o bin/velora-worker ./cmd/worker
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$${VERSION:-dev}" -o bin/velora-migrate ./cmd/migrate
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$${VERSION:-dev}" -o bin/velora-example-settlement ./cmd/example-settlement-service
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$${VERSION:-dev}" -o bin/velora-storage-check ./cmd/storage-check

check: fmt contract
	go vet ./...
	go test ./...
	cd $(WEB_DIR) && $(PNPM) lint
	cd $(WEB_DIR) && $(PNPM) test
	cd $(WEB_DIR) && $(PNPM) build
	node scripts/check-web-bundle-budget.mjs

ai-governance:
	bash scripts/check-ai-governance.sh

ci-policy: ai-governance
	bash scripts/check-ci-policy.sh
	bash scripts/check-container-policy.sh

apisix-policy:
	bash scripts/check-apisix-policy.sh

ci-go: ci-policy contract proto-check module-boundaries
	$(GO_ENV) go mod verify
	@test -z "$$(gofmt -l $$(find cmd internal -name '*.go'))" || (echo "Go files need formatting" >&2; gofmt -l $$(find cmd internal -name '*.go'); exit 1)
	$(GO_ENV) go vet ./...
	$(GO_ENV) go test ./...
	$(GO_ENV) go test -race ./...

ci-web: ci-policy web-install
	cd $(WEB_DIR) && $(PNPM) lint
	cd $(WEB_DIR) && $(PNPM) test
	cd $(WEB_DIR) && $(PNPM) build
	node scripts/check-web-bundle-budget.mjs

ci-web-e2e: ci-web
	$(PNPM) --filter @forge/e2e e2e
	node scripts/check-web-bundle-budget.mjs

ci-deploy: ci-policy apisix-policy pitr-check-test
	bash scripts/check-observability-policy.sh
	helm lint deploy/helm/forge -f deploy/helm/forge/values-xinchuang.yaml
	helm template forge deploy/helm/forge -f deploy/helm/forge/values-xinchuang.yaml >/tmp/forge-rendered.yaml

security-tools: ci-policy
	$(TOOL_RUN) github.com/securego/gosec/v2/cmd/gosec -exclude-generated ./...
	$(TOOL_RUN) golang.org/x/vuln/cmd/govulncheck ./...
	$(TOOL_RUN) honnef.co/go/tools/cmd/staticcheck ./...
	$(TOOL_RUN) github.com/golangci/golangci-lint/v2/cmd/golangci-lint run ./...

supply-chain-evidence: ci-policy
	bash scripts/generate-supply-chain-evidence.sh

release-evidence: supply-chain-evidence
	bash scripts/verify-image-supply-chain.sh

verify: ci-go ci-web ci-deploy security-tools supply-chain-evidence

offline-check: fmt contract
	python3 -m json.tool web/apps/shell/package.json >/dev/null
	bash -n scripts/init-project.sh
	bash scripts/check-offline-package.sh

disaster-check:
	python3 scripts/check-disaster-evidence.py

disaster-check-certified:
	@test -n "$(DR_EVIDENCE_FILE)" || (echo "DR_EVIDENCE_FILE is required" >&2; exit 1)
	python3 scripts/check-disaster-evidence.py --file "$(DR_EVIDENCE_FILE)" --require-certified

docker-build:
	@for value in "$$VELORA_NODE_IMAGE" "$$VELORA_GO_IMAGE" "$$VELORA_RUNTIME_IMAGE"; do \
		[[ "$$value" =~ @sha256:[0-9a-f]{64}$$ ]] || { echo "all VELORA_*_IMAGE values must be internal repository@sha256 references" >&2; exit 1; }; \
	done
	docker build -f deploy/docker/Dockerfile \
		--build-arg NODE_IMAGE="$$VELORA_NODE_IMAGE" \
		--build-arg GO_IMAGE="$$VELORA_GO_IMAGE" \
		--build-arg RUNTIME_IMAGE="$$VELORA_RUNTIME_IMAGE" \
		--build-arg NPM_REGISTRY="$(NPM_REGISTRY)" \
		--build-arg GOPROXY="$(GOPROXY)" \
		--build-arg GOSUMDB="$(GOSUMDB)" \
		-t $(APP):dev .

compose-up:
	docker compose -f deploy/compose/minimal.yaml up -d --build

compose-down:
	docker compose -f deploy/compose/minimal.yaml down -v

init:
	@test -n "$(APP)" || (echo "APP is required" && exit 1)
	@test -n "$(MODULE)" || (echo "MODULE is required" && exit 1)
	./scripts/init-project.sh "$(APP)" "$(MODULE)"
