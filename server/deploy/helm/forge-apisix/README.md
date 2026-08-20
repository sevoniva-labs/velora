# Forge APISIX integration

This optional chart creates APISIX Ingress Controller resources for the Forge HTTP and gRPC endpoints. It does not install APISIX itself and is never required for the modular-monolith baseline. Mirror the approved APISIX and Ingress Controller charts and images into the organization's domestic Helm registry and Harbor before installation; no public-registry fallback is permitted.

## Reviewed compatibility boundary

| Component | Reviewed contract | Enforcement |
|---|---|---|
| APISIX Ingress Controller | `2.1.0` | Pinned in values, schema, annotations, and render guards |
| Kubernetes | `1.31+` | Official controller prerequisite; recorded in values and guards |
| APISIX CRDs | `apisix.apache.org/v2` | Rendered resources must pass the installed 2.1.0 admission schema |

Many domestic financial institutions operate Kubernetes releases older than 1.31. In that case keep this chart disabled. Do not silently install an old controller or assume CRD compatibility. The institution may still operate APISIX as a separately governed gateway, but that path needs its own version matrix, admission test, upgrade rehearsal, and security assessment.

## Prerequisites

- APISIX Ingress Controller 2.1.0 with the `apisix.apache.org/v2` CRDs installed on Kubernetes 1.31 or newer.
- The Forge chart installed in the same namespace and its Service name supplied as `service.name`.
- Existing Kubernetes TLS Secrets for browser ingress, gRPC ingress, and the APISIX client certificate used toward Forge.
- The Forge server configured with TLS and client-certificate verification for APISIX-to-Forge traffic.

Render and review before applying:

```sh
helm template velora-apisix deploy/helm/velora-apisix \
  --namespace velora \
  --set enabled=true \
  --set hosts.web=velora.bank.internal \
  --set hosts.grpc=grpc.velora.bank.internal \
  --set governance.changeApprovalRef=CHG-20260817-001
```

The production profile rejects placeholder hosts, missing approval references, plaintext upstreams, disabled gRPC client mTLS, shared HTTP/gRPC hostnames, and any unreviewed controller version. Run `make apisix-policy` to execute both positive and negative render evidence.

The chart deliberately sets upstream retries to zero. Retrying non-idempotent financial operations at the gateway can duplicate side effects; add application idempotency before changing this policy. The default rate limit is explicitly `local`, so each APISIX instance owns a separate bucket. Replace it with an approved Redis-backed plugin configuration when a cluster-wide quota is required, and inject Redis credentials through the organization's APISIX secret manager rather than Helm values.

The gateway does not replace application authorization, organization isolation, CSRF, idempotency, body-size validation, audit, or stable error-code handling. The `remote_addr` quota is only meaningful after the platform team proves how the load balancer preserves source addresses; otherwise select a reviewed, non-spoofable key at the gateway layer.

## TLS boundary

Ingress TLS terminates at APISIX. APISIX then uses `https` and `grpcs` and presents the client certificate referenced by `upstream.clientCertificateSecretName`; Forge must verify that certificate. APISIX currently does not verify the legality of upstream server certificates, so this is not symmetric end-to-end identity verification. Keep Forge ingress restricted to APISIX Pods with NetworkPolicy, use dedicated short-lived certificates, and record this residual risk in the deployment security assessment.

References:

- https://apisix.apache.org/docs/ingress-controller/getting-started/configure-routes/
- https://apisix.apache.org/docs/ingress-controller/next/reference/apisix-ingress-controller/api-reference/
- https://apisix.apache.org/docs/apisix/FAQ/
