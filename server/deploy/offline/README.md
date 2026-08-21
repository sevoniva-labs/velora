# Offline Delivery Contract

This directory defines the air-gapped delivery contract. It is not a claim that a Harbor mirror, Kubernetes cluster, or target Xinchuang operating system has already been tested.

## Required bundle

An approved release bundle must contain:

- `manifest.sha256`: SHA-256 entries for every file in the bundle
- `provenance.txt`: release commit, tool versions, source mirrors, SBOM and approval references
- `images.lock`: internal OCI references, each pinned with `@sha256:<64 hex>`
- Go module files and sums, the pnpm lockfile, generated API artifacts, Helm charts, configuration profiles, migrations, and signed release metadata

The bundle must be built in a controlled network, mirrored into the organization repository, scanned, signed, and transferred through the institution's approved media process. Runtime pods must not download dependencies or modify application files.

## Verification

Run:

```bash
OFFLINE_BUNDLE_DIR=/path/to/approved-bundle make offline-check
```

The check rejects public OCI registries, missing provenance, unpinned images, and any file whose SHA-256 differs from `manifest.sha256`. Without `OFFLINE_BUNDLE_DIR`, `make offline-check` only checks repository prerequisites and explicitly does not produce offline evidence.

## Status labels

- `Built-in`: repository lockfiles, domestic source policy, digest and manifest verification
- `Profile`: organization Harbor, signing CA, KMS/HSM, Kubernetes runtime, OS and CPU profile
- `Not certified`: a target bundle or air-gapped installation without a dated test report
