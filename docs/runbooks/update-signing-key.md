# Desktop Update Signing Key Runbook

## Purpose

This runbook governs the minisign key used by CompozyOS desktop updater artifacts and `runtime.json`. It also records the release gates for Apple Developer ID, Apple notarization, Azure Artifact Signing, and R2 publication.

The GitHub organization secret is a working copy, not a backup. Losing either the private key or its password strands every installed client that trusts that key.

## Key identity register

The Release Owner updates this table before a key is used. The `keynum` comes from the decoded minisign public key. Never record private-key material here.

| Key | Minisign keynum | Generated (UTC) | First shipped version | Last shipped version | Status |
| --- | --- | --- | --- | --- | --- |
| A | Required before first public release | Required | Required | Open | Current |
| B | Add only during an approved rotation | — | — | — | Not generated |

For every release, attach evidence showing which current and previous public keys were compiled into the app. The release evidence must match this register.

## Custody roster

The Security Owner maintains at least two custodians in different physical locations:

| Role | Holds | Location | Quarterly attestation |
| --- | --- | --- | --- |
| Primary custodian | Encrypted private-key backup | Separate secure location A | Required |
| Recovery custodian | Encrypted private-key backup | Separate secure location B | Required |
| Password custodian | Password record, separate from every key copy | Independent password vault | Required |

Each custodian attests quarterly that the copy is readable, encrypted, access-controlled, and separate from the password. CI secrets, workstation copies, and GitHub organization secrets do not count as backups.

## Generate the first key

Owner: Security Owner. Witness: Release Owner.

1. Use a clean, offline-capable workstation. Confirm `CI` is unset.
2. Set a strong `TAURI_SIGNING_PRIVATE_KEY_PASSWORD` through the approved password manager workflow.
3. Run `scripts/generate-desktop-update-key.sh <encrypted-working-path>` outside CI.
4. Decode the public key, record its `keynum`, date, and witness in the identity register.
5. Create at least two encrypted offline copies in different locations. Store the password separately.
6. Put working copies in `TAURI_SIGNING_PRIVATE_KEY`, `TAURI_SIGNING_PRIVATE_KEY_PASSWORD`, and `TAURI_SIGNING_PUBLIC_KEY` organization secrets.
7. Remove plaintext working files from the generation workstation after the custodians verify their encrypted copies.

The generation script refuses to run when `CI` is set or the password is absent. Never use Tauri's `--ci` key-generation option because it permits an empty password.

## Annual restore drill

Owner: Security Owner. Frequency: annually and before changing custodians.

1. Restore one offline copy onto a clean disconnected workstation.
2. Retrieve the password through the independent password custodian.
3. Sign a disposable file, decode the shipped public key from the latest released app, and verify the signature.
4. Confirm the restored private key's `keynum` equals the key registered for the latest shipped version.
5. Record the drill date, participants, keynum, and verification output in the release security evidence.
6. Delete the restored working copy.

Last successful drill: **must be recorded before the first public release**.

## Planned rotation

Owner: Security Owner. Approvers: Release Owner and Incident Commander.

The Tauri feed carries one current public key. CompozyOS also keeps one previous key as a runtime verification fallback, but already-installed clients can only learn a new key through a release they can verify. Therefore rotation always requires an immortal transitional release.

| Phase | Artifact signed by | App embeds current key | App embeds previous key | Required action |
| --- | --- | --- | --- | --- |
| Before rotation | A | A | none | Keep normal releases available. |
| Transitional release T | A | B | A | Publish T, verify both app and runtime update paths, and never delete T or its payloads. |
| First B release | B | B | A | Publish only after T has passed the full three-platform rehearsal. |
| Later B releases | B | B | A until the approved tail window ends | Monitor support and preserve every A-signed artifact. |
| A retirement | B | B | none | Requires an explicit security decision and documented stranded-tail acceptance. |

**Stranded-tail statement:** any client that does not install transitional release T before releases switch to key B cannot verify a B-signed update and may be permanently stranded. The transitional release and its payloads must never be deleted.

Rotation procedure:

1. Generate B with the same off-CI, password, backup, and restore requirements as A.
2. Add B to the identity register and configure it as the current public key; configure A as the previous public key.
3. Build T with B current and A previous, but sign T's updater artifacts with A.
4. Run the full draft-release rehearsal on macOS, Windows, and Linux. Verify T installs from an A-only client and then accepts both an A fixture and the next B-signed fixture.
5. Publish T. Preserve its GitHub archive and R2 objects permanently.
6. Publish the first B-signed release only after T evidence is approved.

## Key or password loss

Owner: Incident Commander.

There is no automated remediation for loss. Generate a new keypair, ship new installers that embed it, publish an out-of-band security notice, and require manual reinstall for 100% of affected clients. Do not claim that an old installed app can recover through the updater. Preserve old artifacts for forensics and users who still need recovery media.

## Suspected compromise

Owner: Incident Commander. Reference: GHSA-2rcp-jvr4-r259.

1. Freeze release and R2 manifest publication. Do not delete or overwrite existing immutable artifacts.
2. Preserve GitHub, R2, CI, signing, and access evidence.
3. Determine the exposed keynum, versions that embedded it, and every artifact signed with it.
4. If the compromised key still controls clients, use it only under incident approval for one transitional release that embeds the replacement key. Otherwise follow the loss procedure and require manual reinstall.
5. Publish an out-of-band advisory with affected versions, hashes, and reinstall steps.
6. Rotate CI copies, audit custodian access, and document the complete blast radius before resuming release.

## Blast radius

- Update-key compromise can authorize malicious app updater payloads and signed runtime manifests for every client trusting that key.
- Apple or Azure signing compromise can create OS-trusted installers but cannot satisfy minisign verification by itself.
- R2 compromise can deny updates or replay immutable objects, but modified manifests or payloads fail minisign verification unless the update key is also compromised.
- A GitHub compromise affects the archive and release metadata; the updater never references GitHub URLs.
- Key or password loss affects update availability for the entire installed fleet and requires manual reinstall unless a valid transitional release was already installed.

## Launch gate checklist

The Release Owner records a named assignee, completion time in UTC, and evidence link for every row. A missing row blocks publication.

| Gate | Accountable role | Lead time | Required evidence |
| --- | --- | --- | --- |
| Organization secret visibility | Organization Admin | Hours–2 days | Repository access to every Apple, Azure, update-key, and R2 secret |
| macOS environment remap | Release Engineer | 1 hour | Fail-closed preflight output |
| Developer ID certificate and Team ID | Apple Account Holder | 1 day; 1–2 weeks if new | Certificate validity and signing identity |
| ASC API key | Apple Account Holder | 1 hour | Issuer, key ID, and single-download `.p8` custody record |
| First notarization | Release Engineer | 2–6 hours | `notarytool` acceptance and `stapler validate` for `.app` and `.dmg` |
| Azure Artifact Signing identity | Organization Admin | 1–3 days; 1–6 weeks if revalidation is required | Account/profile roles and a signed NSIS artifact |
| `artifact-signing-cli` migration | Release Engineer | 2 hours | Pinned CLI version and object-form bundler command |
| Minisign key ceremony | Security Owner | 1 hour | Key register, witness, password separation |
| Custody review | Security Owner | 1 day | Approved roster and restore procedure |
| Offline backups | Key Custodians | 2–5 days | Two-location attestation |
| R2 bucket/domain/cache policy | Infrastructure Owner | 1 day | Immutable payload and no-cache manifest headers |
| Reserved stable prefix | Infrastructure Owner | 1 hour | No objects under `desktop/stable/` |
| Ubuntu 22.04 runner and fallback | Release Engineer | 1 hour | Runner availability and glibc-floor evidence |
| CDN privacy retention | Privacy Owner | 1 day | Approved log-retention configuration |
| Linux FUSE guidance | Documentation Owner | 2 hours | Published `libfuse2` and extraction fallback guidance |
| Three-platform update rehearsal | QA Owner | 2–3 days | Forced failure leaves app intact; permissions are not `0700`; roll-forward reaches stale clients |

## Release operator sequence

1. Confirm the release channel is `beta`; `desktop/stable/` must remain empty.
2. Confirm the candidate version is strictly greater than the live beta feed.
3. Confirm all platform jobs and signing preflights are green.
4. Confirm exact artifact inventory, every updater signature, runtime manifest signature, app/DMG stapling, and bundle version.
5. Upload immutable payloads first, then `runtime.json`, its minisign signature, and `latest.json` with no-cache headers.
6. Re-fetch the public manifests, download one updater payload per platform, and verify every signature against the shipped public key.
7. Publish the GitHub draft last. GitHub remains an archive and must never appear in feed URLs.

Emergency rollback is roll-forward only: publish the last good content under a new, strictly greater version. Never enable a downgrade comparator or rewrite immutable payloads.

