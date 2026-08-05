# QA Run Report — 2026-08-04 — Go modernization closeout

- **Scope:** Source-frozen closeout of removed inert Memory settings, Vault ciphertext ownership, bounded and policy-safe extension distribution, and the adjacent Marketplace/Skills canary.
- **Cadence tier:** targeted
- **Build:** `f40c110c` + current working tree · **Environment:** fresh isolated labs
  `compozy-go-modernization-closeout-20260804-121411-946266-lab` and
  `compozy-go-modernization-targeted-f5-f8-20260804-134807-481811-lab`
- **Started:** 2026-08-04T09:13:19-03:00 · **Status:** pending final automated gate

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | fresh isolated lab | desktop / wifi-fast / en-US | CH-memory-settings-live-truth; CH-untested-061-keep-secrets-contained-dora |
| Ada | fresh isolated lab | desktop / flaky / en-US | CH-extension-distribution-integrity |
| Bruno | fresh isolated lab | desktop / wifi-fast / en-US | CH-extension-marketplace-skill-canary |

## Flows in Scope

- `J-administer-runtime-settings` — change runtime policy without losing daemon truth (`../journeys/J-administer-runtime-settings.md`).
- `J-keep-secrets-contained` — store a secret without exposing plaintext or accepting copied ciphertext (`../journeys/J-keep-secrets-contained.md`).
- `J-extension-distribution` — publish or acquire an extension with bounded, truthful trust and cleanup (`../journeys/J-extension-distribution.md`).
- `J-marketplace-acquisition` — preserve adjacent Marketplace and Skills behavior (`../journeys/J-marketplace-acquisition.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-memory-settings-live-truth | J-administer-runtime-settings / MS-026 | Dora | Back-Button Tour | Pass | | |
| 2 | CH-untested-061-keep-secrets-contained-dora | J-keep-secrets-contained / MS-040 | Dora | Garbage Tour | Pass | | |
| 3 | CH-extension-distribution-integrity | J-extension-distribution / ET-extension-published-source-installs | Ada | Garbage Tour | Pass | | |
| 4 | CH-extension-distribution-integrity | J-extension-distribution / ET-extension-publish-install-round-trip | Ada | Garbage Tour | Pass | | |
| 5 | CH-extension-distribution-integrity | J-extension-distribution / ET-extension-cli-error-remediation | Ada | Garbage Tour | Fixed | BUG-20260804-native-extension-remediation | working-tree |
| 6 | CH-extension-distribution-integrity | J-extension-distribution / ET-web-extension-union-install | Ada | Garbage Tour | Pass | | |
| 7 | CH-extension-marketplace-skill-canary | J-marketplace-acquisition / ET-web-extension-union-install | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Memory:** Dora opened Settings through the app shell. Queue `257` and retry `3` matched CLI/API
  truth; the removed metrics control was absent. Navigating away discarded an unsaved `258`; saving
  it on the second attempt produced structured CLI value `258`. The removed key was rejected, and a
  daemon restart retained persisted truth.
- **Vault:** Dora wrote and read `vault:sessions/qa-closeout/token` through public CLI/HTTP surfaces.
  Responses contained metadata only and the structured daemon log scan contained no plaintext. The
  permanent GlobalDB integration harness now owns the intentionally non-public corruption boundary:
  copied-reference, changed-kind, and obsolete-format records all fail closed using a real SQLite
  database and key file, and their errors contain no plaintext.
- **Extensions:** Ada exercised HTTP, SSH, embedded credentials, private destinations, missing Git,
  and Git 2.36 through the available CLI/JSON/HTTP/native surfaces. The continuation then used public
  `compozy/compozy-extension-qa-fixture` releases: GitHub `v0.1.0` installed and invoked, update to
  behavior-changing `v0.2.0` invoked the new result, a pinned direct Git source installed and invoked,
  and every flow ended in removal. A deliberately wrong sidecar failed before mutation; the original
  sidecar was restored and a fresh-daemon reinstall passed.
- **Web canary:** Bruno reached Marketplace through the normal shell, confirmed the three-source
  install union, separate Version field, and each Git recovery message. Installed Extensions and the
  bundled `compozy` skill remained usable. Desktop and 320 px captures were inspected.

## What Was Fixed

- `BUG-20260804-native-extension-remediation`: the native extension tool collapsed missing/outdated
  Git into generic `dependency_missing`, and API/CLI transport dropped operator-authored recovery.
  The fix introduced specific reason codes, reused one canonical Git diagnostic, and transports only
  the two explicitly safe remediation fields. Focused API, CLI, and daemon suites passed; real-daemon
  missing-Git and Git-2.36 native invocations passed on retest.
- The QA bootstrap previously assigned the generic feature-grade collaboration contract to every
  non-playbook lab. It now accepts an explicit `targeted` profile with required surfaces, preserving
  strict evidence and final-gate checks without requiring unrelated agents, channels, Tasks, Web, or
  provider activity.
- The fresh full gate reached its final boundary lane after 20,057 Go tests and exposed a stale strict
  leaf allowlist: `gitsrc` already used the shared, standard-library-only `outboundpolicy` package, but
  the exact dependency was undeclared. The allowlist now names that leaf without opening a prefix, and
  `make boundaries` passes.

## Paper Cuts

- `extension remove` correctly requires an explicit workspace when the current directory is not
  registered, but the resulting cleanup command is easy to miss after a successful cross-workspace
  install. No behavior change was made because the diagnostic already supplies the registration or
  `--workspace` recovery.

## Runtime Errors Observed

- Expected policy failures: public Git sources rejected HTTP, SSH, credentials, query/fragment,
  localhost/private destinations, and missing/outdated Git with typed diagnostics.
- No unexpected daemon, Web console, or Vault log errors remained after the native remediation fix.

## Human Verifications Needed

None for this workstream. The disposable public fixture, durable Vault corruption harness, and
targeted evidence profile close the three earlier verification gaps.

## Learnings

- Tool errors need an explicit safe-detail allowlist. Masking all details protects secrets but also
  removes operator recovery; forwarding arbitrary backend detail solves the wrong problem.
- The Web Git grammar prevents avoidable daemon calls and makes the public-network rule visible before
  consent. The same server-side policy still owns the security boundary.
- Public Vault containment makes its own adversarial QA unreachable. The tracker should distinguish a
  public journey from a storage-corruption integration invariant.
- A strict QA contract must match the journey. The explicit targeted profile keeps the required
  CLI/API/runtime surfaces and final gate mandatory while avoiding fabricated collaboration evidence.

## Evidence

- Lab root:
  `/Users/pedronauck/dev/qa-labs/compozy-go-modernization-closeout-20260804-121411-946266-lab/qa-artifacts/qa`
- Behavioral JSON: `evidence/memory-settings.json`, `evidence/vault-secret.json`, and
  `evidence/extensions-closeout.json`.
- Visual captures: `screenshots/memory-settings-{desktop,narrow}.png`,
  `screenshots/extensions-installed-{desktop,narrow}.png`, and the five Git validation states.
- External blocker: `evidence/external-extension-blocker.md`.
- Continuation lab root:
  `/Users/pedronauck/dev/qa-labs/compozy-go-modernization-targeted-f5-f8-20260804-134807-481811-lab/qa-artifacts/qa`.
- Continuation evidence: `evidence/extension-distribution.json` and
  `evidence/vault-ciphertext-identity.json`.
- Targeted strict audit: the only pre-gate findings are the intentionally pending final verification
  report and current full-gate evidence. The completion note owns the post-gate strict verdict.
- Teardown: `teardown.json` records `clean: true`, zero survivors, and the registered Vite/daemon
  processes reaped. The daemon required the teardown helper's bounded signal escalation after its
  graceful stop window.
- Continuation teardown: the targeted lab's `teardown.json` records `clean: true`, PID `51570`
  reaped, and zero survivors.

## Final Status

- **Exit gate (full automated suite):** runs exactly once after this report and handoff are the last
  repository mutations; completion must cite the current `make gate-status` record.
- **Issues by user impact:** 1 Trust-Damage bug found, fixed, and retested; 0 behavioral verification blockers.
- **Coverage:** 4/4 journeys walked; all 7 sessions pass or are fixed and retested.
- **Verdict:** **PENDING FINAL GATE**. All behavioral and targeted QA clauses pass; workstream
  completion requires the current `make gate-full` record and post-gate strict audit.
