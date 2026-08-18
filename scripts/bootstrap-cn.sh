#!/usr/bin/env bash
# ============================================================================
# Velora 中国大陆开发环境引导脚本
#
# 作用（仅影响当前终端会话 / 当前项目，不修改任何全局配置文件）：
#   1. Go Modules 代理：设置 GOPROXY（goproxy.cn），可随时切回官方源
#   2. pnpm 源检查：确保 web/.npmrc 使用国内 registry，并允许切换回官方
#   3. Docker 国内镜像：给出构建/拉取时覆盖 DOCKER_REGISTRY 的用法说明
#
# 用法：
#   ./scripts/bootstrap-cn.sh            # 仅打印配置建议
#   source ./scripts/bootstrap-cn.sh --apply  # 在当前 shell 中临时应用（export 需 source 才生效）
# ============================================================================
set -euo pipefail

apply="${1:-}"

echo "==> Velora 中国大陆开发环境引导"
echo ""

# --- Go Modules 代理 ---
GOPROXY_CN="https://goproxy.cn,direct"
echo "[1/3] Go Modules"
echo "  国内推荐:  GOPROXY=${GOPROXY_CN}"
echo "  官方源:    GOPROXY=https://proxy.golang.org,direct"
if [ "${apply}" = "--apply" ]; then
  export GOPROXY="${GOPROXY_CN}"
  echo "  => 已在本终端临时设置 GOPROXY=${GOPROXY_CN}"
  echo "     （仅当前进程生效，不影响 ~/.zshrc / ~/.bashrc）"
else
  echo "  提示: 如需临时应用，请使用 source 方式执行（export 只在当前 shell 生效）:"
  echo "        source ./scripts/bootstrap-cn.sh --apply"
fi
echo ""

# --- pnpm / npm 源 ---
WEB_NPMRC="web/.npmrc"
NPM_REGISTRY_CN="https://registry.npmmirror.com"
NPM_REGISTRY_OFFICIAL="https://registry.npmjs.org/"
echo "[2/3] pnpm 源（项目级 web/.npmrc，不修改全局配置）"
if [ -f "${WEB_NPMRC}" ]; then
  if grep -q "registry=${NPM_REGISTRY_CN}" "${WEB_NPMRC}" 2>/dev/null; then
    echo "  => ${WEB_NPMRC} 已配置国内源: ${NPM_REGISTRY_CN}"
  else
    echo "  => ${WEB_NPMRC} 存在，但未使用国内源（当前内容如下）"
    cat "${WEB_NPMRC}" | sed 's/^/     /'
  fi
  echo "  切换回官方源: 将 ${WEB_NPMRC} 中 registry 改为 ${NPM_REGISTRY_OFFICIAL}"
else
  echo "  => 未找到 ${WEB_NPMRC}，首次 pnpm install 前请创建，内容:"
  echo "     registry=${NPM_REGISTRY_CN}"
fi
echo ""

# --- Docker 国内镜像 ---
echo "[3/3] Docker 镜像加速"
echo "  构建时指定:  DOCKER_REGISTRY=docker.m.daocloud.io make docker-build"
echo "  或使用官方:  DOCKER_REGISTRY=docker.io make docker-build"
echo "  拉取基础镜像也可先通过加速源 pull 后重新 tag（视本地环境而定）。"
echo ""

echo "==> 完成。日常开发建议:"
echo "    make init && make dev"
echo ""
echo "==> 如需恢复官方源，重新打开终端即可（本脚本不做持久化修改）。"
