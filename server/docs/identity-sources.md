# Federated identity sources

## Status labels

- `Built-in`: the provider-neutral port and security policy are part of the scaffold.
- `Profile`: the provider is enabled by a documented deployment profile.
- `Adapter slot`: a target-specific connector can be added without changing domain code.
- `Target-tested`: the exact issuer/directory version, TLS chain, browser flow, and failure cases passed the target contract suite.
- `Not certified`: no target evidence exists; production approval must not infer support from protocol compatibility.

## Built-in security behavior

- OIDC uses HTTPS discovery, authorization-code exchange, ID-token verification, nonce checking, and a one-time Redis/cache-backed state with a five-minute TTL.
- LDAP and Active Directory use the LDAP adapter. LDAPS or StartTLS is required by default; plaintext LDAP requires an explicit non-production override.
- External authentication never provisions a local user and never matches by email, login name, or display name.
- A local account must have an explicitly approved `(organization, provider, subject)` mapping before a federated session can be created.
- Federated sessions are marked `FEDERATED`, not `MFA`. Privileged actions still require the existing recent local MFA step-up.
- Missing provider configuration, cache state, mapping, or target authentication evidence fails closed.

## Environment configuration

OIDC is enabled only when `VELORA_OIDC_ISSUER` is set. Required values are `VELORA_OIDC_NAME`, `VELORA_OIDC_CLIENT_ID`, `VELORA_OIDC_CLIENT_SECRET` or `VELORA_OIDC_CLIENT_SECRET_FILE`, and `VELORA_OIDC_REDIRECT_URL`.

### Casdoor profile

Casdoor is used unchanged as a standard external OIDC issuer. Register a confidential client in its existing administration plane and set its redirect URI to `https://<velora-host>/api/v1/auth/federated/oidc/casdoor/callback`. The backend accepts the OIDC authorization-server browser `GET` callback (and retains the JSON `POST` callback for API clients), exchanges the authorization code server-side, and validates issuer, signature, audience, nonce, and one-time state.

This integration does not call Casdoor management APIs, use resource-owner-password login, or turn Forge into an OIDC provider. Authentication is bound only through the immutable `sub` claim and an approved local `(organization, provider, subject)` mapping. Portal resource permissions remain local policy data until a separately approved, tested Casdoor claim-to-role mapping is implemented; raw `groups`/`roles` claims must not be trusted implicitly.

LDAP/AD is enabled only when `VELORA_LDAP_URL` is set. Required values are `VELORA_LDAP_NAME`, `VELORA_LDAP_BIND_DN`, `VELORA_LDAP_BIND_PASSWORD` or `VELORA_LDAP_BIND_PASSWORD_FILE`, `VELORA_LDAP_BASE_DN`, and `VELORA_LDAP_LOGIN_ATTRIBUTE`.

`VELORA_LDAP_STARTTLS=true` enables StartTLS for an `ldap://` endpoint. `VELORA_LDAP_ALLOW_INSECURE=true` is an explicit development-only escape hatch and must not be used in a production profile.

Provider credentials are environment/file-only secrets. Do not place them in YAML, Helm values, Git, approval payloads, or audit details.

## Evidence boundary

The scaffold provides the adapter and policy boundary. It does not certify a commercial IAM, OIDC issuer, LDAP/AD version, CA chain, browser fleet, or high-availability topology. Each target must record discovery, code exchange, nonce/state replay, TLS failure, directory timeout, disabled-account, unmapped-subject, MFA step-up, failover, and audit evidence before being labeled `Target-tested`.
