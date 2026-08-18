# ============================================================================
# Velora Makefile
#
# 常用流程：
#   make init        # 准备环境（Go 代理 / pnpm 源 / 依赖 / .env）
#   make dev-web     # 前端开发（Vite, :5173, 代理 /api → :8080）
#   make dev-server  # 后端开发（Gin, :8080）
#   make docker-up   # 一键起 PostgreSQL + Casdoor + Velora Server + Web
# ============================================================================

SHELL := /bin/bash
# Homebrew 工具链兜底（macOS 下 go 常位于此；该目录不存在时不影响其它平台）
export PATH := /opt/homebrew/bin:$(PATH)
DOCKER_REGISTRY ?= docker.m.daocloud.io

.PHONY: help init bootstrap dev dev-web dev-server test lint build \
        docker-build docker-up docker-down migrate seed fmt vet

help: ## 显示可用命令
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

bootstrap: ## 中国大陆开发环境引导（Go 代理 / pnpm 源 / Docker 镜像说明）
	./scripts/bootstrap-cn.sh

init: bootstrap ## 初始化：依赖安装 + .env 准备
	@if [ ! -f .env ]; then cp .env.example .env && echo "=> 已从 .env.example 创建 .env（请按需修改）"; else echo "=> .env 已存在，跳过"; fi
	cd server && go mod download
	cd web && pnpm install

dev: ## 同时启动前后端开发（Ctrl+C 停止）
	$(MAKE) -j2 dev-web dev-server

dev-web: ## 前端开发服务器（:5173）
	cd web && pnpm dev

dev-server: ## 后端开发服务器（:8080）
	cd server && go run ./cmd/velora serve

test: ## 全部测试（后端 go test + 前端 lint/build/test）
	cd server && go test ./...
	cd web && pnpm lint && pnpm test && pnpm build

lint: ## 代码检查（go vet + oxlint）
	cd server && go vet ./...
	cd web && pnpm lint

fmt: ## 格式化（gofmt + goimports 风格由 gofmt 处理）
	cd server && gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

build: ## 构建（server 二进制 + web 产物）
	cd server && go build -o bin/velora ./cmd/velora
	cd web && pnpm build

docker-build: ## 构建 Docker 镜像（可用 DOCKER_REGISTRY=docker.io 切官方源）
	DOCKER_REGISTRY=$(DOCKER_REGISTRY) docker compose build

docker-up: ## 启动全部服务（PostgreSQL + Casdoor + Server + Web）
	DOCKER_REGISTRY=$(DOCKER_REGISTRY) docker compose up -d

docker-down: ## 停止全部服务
	docker compose down

migrate: ## 执行数据库迁移（本地直连 .env 中的 DATABASE_URL）
	cd server && go run ./cmd/velora migrate

seed: ## 写入开发 Seed 数据（分类 + 示例应用）
	cd server && go run ./cmd/velora seed
