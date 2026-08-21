ARG NODE_IMAGE
ARG GO_IMAGE
ARG RUNTIME_IMAGE

FROM ${NODE_IMAGE} AS web-build
ARG NPM_REGISTRY=https://registry.npmmirror.com
ENV COREPACK_NPM_REGISTRY=${NPM_REGISTRY}
WORKDIR /src
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml .npmrc ./
COPY web/apps/shell/package.json web/apps/shell/package.json
COPY web/apps/platform-admin/package.json web/apps/platform-admin/package.json
COPY web/packages/app-contract/package.json web/packages/app-contract/package.json
COPY web/packages/api-client/package.json web/packages/api-client/package.json
COPY web/packages/auth-sdk/package.json web/packages/auth-sdk/package.json
COPY web/packages/design-system/package.json web/packages/design-system/package.json
COPY web/packages/host-sdk/package.json web/packages/host-sdk/package.json
COPY web/packages/manifest-security/package.json web/packages/manifest-security/package.json
COPY web/packages/microapp-runtime/package.json web/packages/microapp-runtime/package.json
COPY web/packages/observability/package.json web/packages/observability/package.json
COPY web/packages/testkit/package.json web/packages/testkit/package.json
RUN corepack enable && pnpm install --frozen-lockfile --registry=${NPM_REGISTRY}
COPY web/ web/
RUN pnpm --filter @velora/shell build

FROM ${GO_IMAGE} AS go-build
ARG VERSION=container
ARG GOPROXY=https://goproxy.cn
ARG GOSUMDB="sum.golang.org https://goproxy.cn/sumdb/sum.golang.org"
ENV GOPROXY=${GOPROXY} GOSUMDB=${GOSUMDB}
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/velora ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/velora-worker ./cmd/worker \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/velora-migrate ./cmd/migrate \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/velora-storage-check ./cmd/storage-check

FROM ${RUNTIME_IMAGE}
# The signed enterprise runtime base must already contain CA certificates and timezone data.
RUN mkdir -p /app/web /app/data /tmp/velora-nacos \
 && chown -R 10001:10001 /app /tmp/velora-nacos
WORKDIR /app
COPY --from=go-build /out/velora /app/velora
COPY --from=go-build /out/velora-worker /app/velora-worker
COPY --from=go-build /out/velora-migrate /app/velora-migrate
COPY --from=go-build /out/velora-storage-check /app/velora-storage-check
COPY --from=web-build /src/web/dist /app/web
COPY configs/minimal.yaml /app/config.yaml
USER 10001
EXPOSE 8080
ENV VELORA_CONFIG=/app/config.yaml VELORA_WEB_DIR=/app/web
ENTRYPOINT ["/app/velora"]
