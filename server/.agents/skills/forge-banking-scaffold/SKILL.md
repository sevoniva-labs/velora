---
name: forge-banking-scaffold
description: Enforce the Forge enterprise, banking, financial-security, Xinchuang, domestic-source, modular-monolith, Kratos, React, Wujie, generic-S3, evidence, validation, and Git workflow. Use for every architecture, code, configuration, dependency, schema, API, frontend, infrastructure, security, test, documentation, release, provider, migration, or review task in this repository.
---

# Forge Banking Scaffold

Apply this workflow before making repository changes.

## 1. Load authority

1. Read `AGENTS.md` and `docs/ai-engineering-governance.md`.
2. Read `docs/banking-production-feasibility-plan.local.md` when it exists, but never stage or commit it.
3. Read the active Goal objective when the task is Goal-driven.
4. Treat code and executable checks as current-state evidence. Treat prose as intent until verified.

## 2. Protect the worktree

1. Inspect the branch and working tree before editing; stay on a `codex/` branch until final integration.
2. Preserve all pre-existing changes. Stop if unexpected external changes appear.
3. Never use `reset --hard`, checkout-based rollback, rebase, force push, amend, `--no-verify`, or test weakening.
4. Never commit secrets, caches, generated runtime artifacts, local plans, or Goal prompts.

## 3. Classify the slice

- Phase 0: security, reproducibility, supply chain, TLS, policy floors.
- Phase 1: Kratos contracts, modular boundaries, messaging, discovery, gateway, observability, SPA or microfrontend infrastructure.
- Phase 2: identity, organization, authorization, approval, audit, and banking governance.
- Phase 3: data governance, crypto, generic S3, file quarantine, and operations.
- Phase 4: deployment hardening, Xinchuang profiles, offline delivery, disaster recovery, and evidence packs.

Keep changes in the smallest coherent phase slice. Do not mix unrelated backend, frontend, deployment, and documentation work in one commit.

## 4. Enforce architecture

- Default to one modular-monolith process and one React SPA.
- Keep `domain` and `application` free of Kratos, HTTP, persistence, middleware, and vendor SDK imports.
- Put composition in bootstrap/platform layers and external behavior behind ports/providers.
- Use Kratos v2, Protobuf, Buf, OpenAPI, Nacos 3 outside Kubernetes, Kubernetes Service/DNS inside Kubernetes, RocketMQ 5, APISIX adapters, `database/sql`, Goose, Redis-protocol providers, AWS SDK for Go v2 S3, OTLP, React 19, TypeScript 6, Ant Design 6, Vite 8, TanStack Query, pnpm, Helm, and Kubernetes.
- Keep Wujie optional and disabled by default. Run untrusted applications in independent-origin iframes.
- Do not add a technology listed as forbidden in the governance document.

## 5. Enforce banking controls

- Authorize every API, token, batch, worker, and microfrontend action on the backend.
- Apply permission, data scope, grant ceiling, segregation of duties, approval state, and subject/session state where applicable.
- Bind approvals to an immutable request digest and re-authorize at execution time.
- Write critical business state and its local audit/reliable record in one transaction.
- Fail closed when security policy, approval, audit, crypto, storage retention, or evidence prerequisites fail.
- Never log credentials, tokens, private keys, secrets, unmasked regulated data, or sensitive request bodies.
- Require idempotency for repeatable external writes and bound every external timeout/retry.

## 6. Negotiate providers and claims

- Discover S3 capabilities explicitly: STS, multipart, checksum, SSE-S3, SSE-KMS, versioning, object lock, retention, legal hold, presign restrictions, and audit.
- Refuse an operation when its required capability is absent. Do not emulate a compliance guarantee silently.
- Keep provider behavior behind a contract suite and record the exact vendor/product/version/profile.
- Label untested targets as `Adapter slot`, `Experimental`, or `Not certified`.
- Never claim supported, production-ready, financial-grade, banking-grade, or Xinchuang-compatible without target evidence.

## 7. Preserve domestic and offline delivery

- Use configured internal repositories first, then verified domestic mirrors.
- Default Go modules to `https://goproxy.cn` without `direct`; default npm/pnpm to `https://registry.npmmirror.com`.
- Require internal Harbor image references pinned by digest and centralize source configuration.
- Never silently contact foreign public services during build or release.
- Preserve TLS, checksum, signature, SBOM, license, vulnerability, secret, and provenance verification in connected and offline modes.

## 8. Validate proportionally

- AI policy: `make ai-governance`.
- Go/domain: focused tests, then `make ci-go` at the phase gate.
- Contracts: `make proto-check` and `make contract`.
- Frontend: focused package checks, then `make ci-web`; use `make ci-web-e2e` for browser, permission, Wujie, iframe, fallback, or rollback behavior.
- Deployment: `make ci-deploy` plus affected profile/runtime checks.
- Security/dependencies: affected checks, then `make security-tools` and supply-chain evidence at the phase gate.
- Final completion: `make verify` plus external provider, offline, restore, DR, architecture, and target-system evidence required by the Goal.

Do not claim a gate passed unless current output proves its scope. Record unavailable external environments as explicit evidence gaps.

## 9. Commit and hand off

1. Inspect only the intended diff and validation result.
2. Commit the verified coherent slice immediately with Conventional Commits.
3. Report SHA, scope, gates passed, and evidence gaps.
4. Keep the Goal active while any required implementation or real-environment evidence is missing.
5. Merge into `main` only after requirement-by-requirement audit, full validation, and a clean worktree; use a non-destructive merge.
