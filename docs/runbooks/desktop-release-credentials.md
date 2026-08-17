# Desktop release credential custody

The desktop release workflow is the only place that may use production publication and signing credentials. Local QA, draft rehearsals, packages, logs, artifacts, and operator homes must never contain them.

## Credential inventory

- `RELEASE_TOKEN`: GitHub release asset and `channel-beta` authority; it is also the fallback
  web-assets publication authority when the dedicated token is unavailable.
- `COMPOZY_WEB_ASSETS_TOKEN`: preferred web-assets module publication authority for an explicit
  release whose source branch has a stale deterministic Web build pin.
- `APPLE_CERTIFICATE` and `APPLE_CERTIFICATE_PASSWORD`: macOS signing identity.
- `APPLE_API_KEY`, `APPLE_API_KEY_ID`, and `APPLE_API_ISSUER`: Apple notarization authority.
- GitHub Actions OIDC: keyless Sigstore identity for signed compatibility catalogs; there is no exported private key to back up.

Store the named secrets only in the protected GitHub Actions environment. Keep the Apple certificate and API-key source material in the organization's approved encrypted backup system, with recovery access held by at least two authorized release administrators. Record owners, expiry dates, and the last restore check without recording secret values.

## Access review

Before a public beta:

1. Confirm the protected environment requires approval and grants access only to current release administrators.
2. Confirm branch and tag protections still constrain the release workflow.
3. Confirm the encrypted backup inventory can restore the Apple signing identity and API key.
4. Run the release rehearsal without production secrets and prove it fails before build or publication.

## Rotation

Rotate a credential immediately after suspected exposure, an administrator departure, or an authority change. Otherwise rehearse rotation with non-production material at least once per quarter.

1. Create the replacement in its owning service and add it to the protected environment.
2. Run a draft release and verify code signing, notarization, catalog signature, immutable assets, and channel CAS without moving the production channel.
3. Revoke the old credential only after the draft evidence passes.
4. Refresh the encrypted backup and access record, then record the rotation date and approvers.

If verification fails, leave the production channel unchanged, revoke any exposed credential, preserve the audit evidence, and stop the release.
