# QA Run Report — 2026-08-07 — Release Cosign v3

- **Scope:** Repair the release and public-installer trust chain after the beta.6 production failure.
- **Cadence tier:** targeted
- **Build:** `b293d48f` + working tree · **Environment:** fresh targeted QA lab; real GitHub release assets; no publication permissions exercised
- **Started:** 2026-08-07T17:05:37Z · **Status:** blocked

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Release administrator | desktop / wifi-fast / en-US | CH-compozy-beta-candidate, CH-untested-057-evaluate-compozy-beta-dora |

## Flows in Scope

- `J-approve-compozy-beta-candidate` — Prove the candidate trust chain without publishing (`../journeys/J-approve-compozy-beta-candidate.md`)
- `J-evaluate-compozy-beta` — Install a published beta through the provenance-verified path (`../journeys/J-evaluate-compozy-beta.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-compozy-beta-candidate | J-approve-compozy-beta-candidate / REL-release-candidate-plan | Dora | Garbage Tour | Blocked (needs human verify) | GitHub-hosted release-PR dry-run required | |
| 2 | CH-untested-057-evaluate-compozy-beta-dora | J-evaluate-compozy-beta / REL-beta-installer-provenance | Dora | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-compozy-beta-candidate — Dora

- **Ran:** 2026-08-07T17:05:37Z → 2026-08-07T17:09:44Z (box respected: yes)
- **Findings:** The local release contract, Cosign-before-GoReleaser ordering, both workflow pins, preflight regression, and official bundles passed. The GitHub-hosted dry-run is the only unavailable boundary.
- **Bugs filed/updated:** none
- **Scenarios settled:** REL-release-candidate-plan → blocked-verify
- **Paper cuts:** none
- **Surprises:** beta.6 remains absent from both remote tag names, so no cleanup is needed.
- **Suggested next charter:** Re-run this charter on the release PR and capture `Checksum verified` followed by `cosign signature verified`.

### CH-untested-057-evaluate-compozy-beta-dora — Dora

- **Ran:** 2026-08-07T17:05:37Z → 2026-08-07T17:09:44Z (box respected: yes)
- **Findings:** The candidate installer used pinned Cosign v3.1.3 to verify the real beta.5 Sigstore bundle and archive, then installed a binary that independently reported the same version.
- **Bugs filed/updated:** none
- **Scenarios settled:** REL-beta-installer-provenance → pass
- **Paper cuts:** none
- **Surprises:** the deployed hosted script correctly remains on the prior Cosign v2.2.4 source until this candidate ships; the candidate was exercised against the same public beta.5 assets.
- **Suggested next charter:** Repeat the hosted entry point after deployment to replace candidate-build evidence with deployed evidence.

## What Was Fixed

No QA-session fixes.

## Paper Cuts

None recorded.

## Runtime Errors Observed

None recorded.

## Human Verifications Needed

- [ ] On the release PR, confirm the Setup Release Tools step logs `Checksum verified` and `cosign signature verified` before GoReleaser starts (row #1).

## Decisions for a Human

None.

## Learnings

- GoReleaser Pro v2.17.1 and Compozy beta.5 use different Sigstore bundle generations; Cosign v3.1.3 verifies both with the existing identities and issuer.
- Rejecting an invalid release tag before downloads leaves no partial installation state.

## Final Status

**BLOCKED**

The release trust-chain checks, public-installer contract, live beta.5 install, and real Sigstore
bundle verification passed. The repository-wide lint blocker was repaired at its token-source root:
the deck minimum widths now use Tailwind's `--min-width-*` namespace, the generated CSS contains the
bare minimum-width utilities, and the Web test and production-build lanes pass. The GitHub-hosted
release-PR dry-run remains the sole recorded human verification boundary.

Final make verify evidence: /Users/pedronauck/dev/qa-labs/compozy-release-cosign-v3-20260807-170358-478309-lab/qa-artifacts/qa/evidence/final-make-verify.log
