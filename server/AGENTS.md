# Forge Banking Scaffold Agent Contract

These rules are mandatory for every AI agent and repository change.

## Load before work

1. Read `.agents/skills/forge-banking-scaffold/SKILL.md` and follow its workflow.
2. Read `docs/ai-engineering-governance.md` as the authoritative architecture, security, evidence, and delivery policy.
3. If present, read the Git-ignored `docs/banking-production-feasibility-plan.local.md` for the current implementation baseline. Never commit it.
4. For a Goal-driven task, read the attached Goal objective and preserve its full scope.

## Non-negotiable rules

- Build a modular monolith by default. Split a domain only after its documented admission gate passes.
- Use the fixed stack. Do not introduce a forbidden framework or alternative path before the current Goal is complete and a later ADR is approved.
- Keep domain and application code independent of Kratos, transport, database drivers, middleware, and vendor SDKs.
- Enforce permissions, data scope, approval, and audit on the backend. Frontend checks are never an authority boundary.
- Fail closed for security, approval, audit, crypto, storage-retention, and production-policy failures.
- Treat S3 as a capability-negotiated protocol. Never assume MinIO behavior or claim an untested provider is supported.
- Use configured domestic or internal sources without silent foreign fallback. Preserve TLS, hashes, signatures, SBOM, license, vulnerability, and provenance controls.
- Use `Built-in`, `Profile`, `Adapter slot`, `Experimental`, `Target-tested`, and `Not certified` exactly as defined by the governance document.
- Preserve user changes. Never use destructive Git operations, skip hooks, weaken tests, or hide failed validation.
- Complete the smallest coherent slice, run its required gates, and create a Conventional Commit immediately. Do not amend or rebase.

## Required gate

Run `make ai-governance` after changing any AI rule, Skill, technology policy, source policy, or validation entrypoint. Run the validation matrix required by the Skill before committing.
