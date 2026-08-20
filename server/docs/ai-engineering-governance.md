# AI Engineering Governance

This is the authoritative policy for human and AI contributors. Agent files and Skills load this policy; executable checks are the final evidence of implementation state.

## 1. Product boundary

Forge is a new enterprise and banking application scaffold without legacy compatibility obligations. It provides secure defaults, auditable controls, offline delivery, provider boundaries, and evidence generation. A feature switch may disable a complete module; it must not expose a partially governed variant that omits mandatory controls.

The defaults are a modular monolith, one React SPA, optional Wujie behind a governed manifest, and independent-origin iframe isolation for untrusted applications. A domain is split only after ownership, data, SLO, deployment, transaction, and operational admission gates are documented and tested.

## 2. Fixed technology path

- Go 1.26 current security patch and Kratos v2 current stable patch.
- Kratos HTTP/gRPC, Protobuf, Buf, OpenAPI, built-in RBAC/data scope, and a Casbin adapter.
- Kubernetes Service/DNS in Kubernetes; Nacos 3 outside Kubernetes.
- RocketMQ 5 for business messages; Kafka only as a data-stream provider.
- Local transactions first; RocketMQ transaction messages, local reliable messages, or explicit TCC/Saga only for proven scenarios.
- APISIX behind a gateway adapter; `database/sql`, repositories, Goose, Redis-protocol providers, AWS SDK for Go v2 S3, OTLP, and Prometheus.
- React 19, TypeScript 6, Ant Design 6, Vite 8, TanStack Query, pnpm, optional Wujie, Kubernetes, Helm, internal Harbor, Jenkins/GitLab CI, and offline bundles.

Forbidden until this Goal is complete and a later ADR is approved: Kratos v3, go-zero, Hertz, Kitex, Dubbo, Thrift, Apollo, Polaris, Seata, qiankun, MicroApp, Garfish, Module Federation, Turborepo, Nx, and Service Mesh.

## 3. Layer and module rules

- Domain owns invariants and cannot import transports, frameworks, persistence, middleware, or vendor SDKs.
- Application owns use-case orchestration, authorization and approval requirements, transaction boundaries, and ports.
- Repository/provider adapters own SQL, protocols, SDKs, and product differences.
- Transport maps contracts and identity to application commands; bootstrap/platform owns Kratos composition and lifecycle.
- Cross-module writes use an explicit application service or reliable event, never direct cross-owned table access.
- Every external call carries context, bounded timeout, TLS verification, and bounded retry only when safe or idempotent.

## 4. Platform-management completeness

The platform feature consists of organization, department, position, user, user group, role, permission, menu, data scope, session, device, token, MFA, approval, audit, password security, and platform configuration. Enabling it requires all applicable backend controls even when a UI page is disabled.

Identity includes local login plus OIDC and LDAP/AD adapters; joiner/mover/leaver lifecycle; MFA enrollment/recovery/step-up; trusted-device policy; session revocation; grant ceiling; segregation of duties; forbidden combinations; temporary grants; emergency accounts; access review; and scoped, expiring, rotating machine tokens.

## 5. Banking approval and audit invariants

- Sensitive commands require policy-selected maker-checker, countersign or any-sign approval, with timeout, transfer, withdrawal, and immutable request-digest binding.
- Execution rechecks actor, subject, permission, data scope, segregation, approval state, request digest, and resource version.
- Privilege elevation, password reset, regulated-data export, key operations, production configuration, release, job, backup, restore, and emergency access are approval-controlled.
- Critical state and its local audit or reliable journal record commit atomically.
- Audit includes actor, subject, organization, action, resource, result, reason, correlation, source, policy/approval references, and protected before/after digests.
- Integrity chains, WORM retention, recoverable collection, export evidence, and a SIEM adapter are required. Silent audit loss is forbidden.

## 6. Data and cryptography

Maintain data catalog, owner, classification, field tags, purpose, residency, retention, dynamic masking, governed export, watermark, deletion, and evidence. Use a Crypto Provider for envelope encryption, key versions, rotation, dual control, and KMS/HSM adapters. Algorithm labels or software crypto do not prove a certified commercial-cryptography deployment.

## 7. Generic S3 contract

S3 compatibility is capability-based, not product-name-based. Providers report and contract-test STS, temporary credentials, multipart and recovery, checksums, SSE-S3, SSE-KMS, versioning, delete markers, object lock, governance/compliance retention, legal hold, constrained presigning, audit correlation, and failure recovery.

Operations declare required capabilities and fail closed when absent. Uploads enter quarantine and receive server-side type/content/size checks and malware scanning before governed promotion. AWS S3, MinIO, Ceph RGW, Alibaba OSS, Tencent COS, and Huawei OBS remain `Not certified` until an exact target environment passes the suite.

## 8. Domestic sources and offline mode

Use internal Nexus/Artifactory/Athens/Harbor/npm/OS snapshots first, then verified domestic mirrors. Defaults are `https://goproxy.cn` with no `direct` fallback and `https://registry.npmmirror.com`. Import OCI inputs to internal Harbor and pin them by digest. Pin and cache all tools, generators, browser artifacts, scanners, databases, and OS packages. Connected and offline modes must not silently access foreign public services, and both preserve TLS, hash, signature, SBOM, license, vulnerability, secret, and provenance checks.

## 9. Claim and evidence vocabulary

- `Built-in`: implemented here and covered by repository automation.
- `Profile`: supplied configuration/policy; target evidence may remain required.
- `Adapter slot`: interface and extension point only.
- `Experimental`: implemented but not approved for production.
- `Target-tested`: exact product, version, architecture, configuration, date, suite, result, and evidence digest recorded.
- `Not certified`: target evidence is absent, stale, failed, or outside the matrix.

Only `Target-tested` evidence supports a target compatibility claim. Unit tests alone cannot prove financial-grade, banking-grade, Xinchuang, HSM/KMS, vendor-S3, DR, capacity, or long-run production claims.

## 10. Validation and evidence

Each slice runs focused tests before its Conventional Commit. Phase gates run repository CI entrypoints. Completion requires `make verify`, race/static/security checks, frontend production E2E, contracts, generated-code checks, Helm/security policy checks, connected/offline builds, migration/backup/restore, provider contracts, and target DR/compatibility exercises.

Evidence binds source commit, dependency/image/configuration digests, tool versions, environment identity, time range, command, result, signer, and artifact digest. Missing external environments remain explicit gaps and keep the Goal active.

## 11. Git and AI enforcement

Preserve user work; use small independently reversible Conventional Commits; never amend, rebase, force, skip hooks, weaken tests, or commit secrets/local plans/Goal prompts/artifacts. Merge `main` non-destructively only after full requirement evidence and a clean tree.

Codex uses `AGENTS.md` and `.agents/skills`; Claude imports `AGENTS.md` through `CLAUDE.md` and uses `.claude/skills`; Cursor always loads `.cursor/rules/velora-banking-scaffold.mdc`. `scripts/check-ai-governance.sh` prevents drift. No file can control a client that deliberately ignores project instructions, so supported clients use both automatic loading and a CI rejection gate.
