# Crypto and Key-Management Boundary

The scaffold now provides:

- `Built-in`: envelope encryption with a fresh AES-256 data key per payload
- `Built-in`: authenticated data binding (AAD) and provider-controlled KEK versioning
- `Built-in`: versioned `standard` AES-GCM and `gm` SM4-GCM provider baselines
- `Adapter slot`: KMS/HSM/password-device activation through `KeySource`
- `Adapter slot`: two-person approval through `DualControl`

`KeyManager.Rotate` refuses missing approvals, missing identities, identical operators, malformed versions, or an adapter approval failure. It activates no key before dual control succeeds.

The KMS/HSM adapter is intentionally not vendor-specific. Production certification still requires a real target device, key custody policy, backup/restore test, rotation drill, TLS/certificate validation, SM2 certificate/signature verification where required, and an institution-approved audit trail. Until those artifacts exist the target remains `Not certified`; the local provider tests must not be used as a password-device certification.
