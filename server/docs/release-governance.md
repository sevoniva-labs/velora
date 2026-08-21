# Release governance

`internal/platform/releasecontrol` is the common release admission and rollback state machine for regulated environments.

Every release binds an artifact digest, SBOM digest, source commit, provenance evidence, test evidence, vulnerability result, license result, rollback plan, target environment, and approved change window. Production release approval requires at least two distinct approvers, and the requester cannot approve the release or its rollback.

The store adapter must persist the state transition and its audit event in one local database transaction. A controller that updates deployment state and writes audit logs independently is not an acceptable production adapter. External deployment systems remain adapter slots and must return deployment or rollback evidence before the state can advance.

The package is a governance gate, not a certification claim. Target-specific CI/CD, APISIX, Kubernetes, database, message, KMS/HSM, S3, disaster-recovery, and Xinchuang evidence must still be collected by the delivery environment.
