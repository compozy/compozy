# QA Run Report — 2026-08-07 — Release Cosign v3

- **Scope:** Repair the release and public-installer trust chain after the beta.6 production failure.
- **Cadence tier:** targeted
- **Build:** `f8117d9d` + working tree · **Environment:** fresh targeted QA lab plus hosted beta.6 production run
- **Started:** 2026-08-07T17:05:37Z · **Status:** blocked

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Release administrator | desktop / wifi-fast / en-US | CH-compozy-beta-candidate, CH-untested-057-evaluate-compozy-beta-dora, CH-published-npm-channel-readiness |

## Flows in Scope

- `J-approve-compozy-beta-candidate` — Prove the candidate trust chain without publishing (`../journeys/J-approve-compozy-beta-candidate.md`)
- `J-evaluate-compozy-beta` — Install a published beta through the provenance-verified path (`../journeys/J-evaluate-compozy-beta.md`)
- `J-publish-compozy-beta` — Publish one immutable beta and close only after its public channel policy is observable (`../journeys/J-publish-compozy-beta.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-compozy-beta-candidate | J-approve-compozy-beta-candidate / REL-release-candidate-plan | Dora | Garbage Tour | Blocked (needs human verify) | GitHub-hosted release-PR dry-run required | |
| 2 | CH-untested-057-evaluate-compozy-beta-dora | J-evaluate-compozy-beta / REL-beta-installer-provenance | Dora | Feature Tour | Pass | | |
| 3 | CH-published-npm-channel-readiness | J-publish-compozy-beta / REL-published-npm-channel-readiness | Dora | Network Tour | Blocked (needs human verify) | BUG-20260807-npm-dist-tag-readiness | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-compozy-beta-candidate — Dora

- **Ran:** 2026-08-07T17:05:37Z → 2026-08-07T17:09:44Z (box respected: yes)
- **Findings:** The local release contract, Cosign-before-GoReleaser ordering, both workflow pins, preflight regression, and official bundles passed. Hosted beta.6 independently proved the production Cosign path, but it does not replace the release-PR dry-run promised by this pre-publish journey.
- **Bugs filed/updated:** none
- **Scenarios settled:** REL-release-candidate-plan → blocked-verify
- **Paper cuts:** none
- **Surprises:** none
- **Suggested next charter:** Re-run this charter on the next release PR and capture both GoReleaser download verification messages before the dry-run starts.

### CH-untested-057-evaluate-compozy-beta-dora — Dora

- **Ran:** 2026-08-07T17:05:37Z → 2026-08-07T17:09:44Z (box respected: yes)
- **Findings:** The candidate installer used pinned Cosign v3.1.3 to verify the real beta.5 Sigstore bundle and archive, then installed a binary that independently reported the same version.
- **Bugs filed/updated:** none
- **Scenarios settled:** REL-beta-installer-provenance → pass
- **Paper cuts:** none
- **Surprises:** the deployed hosted script correctly remains on the prior Cosign v2.2.4 source until this candidate ships; the candidate was exercised against the same public beta.5 assets.
- **Suggested next charter:** Repeat the hosted entry point after deployment to replace candidate-build evidence with deployed evidence.

### CH-published-npm-channel-readiness — Dora

- **Ran:** 2026-08-07T18:37:02Z → 2026-08-07T18:59:00Z (box respected: yes)
- **Findings:** Hosted beta.6 completed GoReleaser, GitHub publication, CLI publication, and extension SDK publication. The verifier ran about two seconds after npm accepted the SDK, observed beta.5 once, and failed; later public reads showed both packages on beta.6 without a republish.
- **Bugs filed/updated:** BUG-20260807-npm-dist-tag-readiness
- **Scenarios settled:** REL-published-npm-channel-readiness → blocked-verify
- **Paper cuts:** none
- **Surprises:** the failed run still produced a correct `v0.3.0-beta.6` GitHub prerelease and correct beta.6 npm channels; the red job represented acceptance timing, not a failed publication.
- **Suggested next charter:** Re-run this charter on the next hosted beta and capture each readiness attempt plus the final fresh registry state.

## What Was Fixed

### BUG-20260807-npm-dist-tag-readiness: A published beta can fail while npm channel metadata propagates

- **Symptom:** the production job failed after every immutable artifact had published successfully.
- **Root cause:** one immediate npm dist-tag query treated eventually consistent registry metadata as a terminal policy result.
- **Fix:** working tree — bounded condition-based readiness for both public packages, with fresh reads and immediate failure for terminal errors.
- **Regression test:** `internal/config/release_config_test.go::TestNPMReleasePolicyWaitsForRegistryReadiness` — failed before the helper existed; passes for convergence, timeout evidence, and terminal policy rejection.
- **Retested:** hosted retest pending; local integration boundary passed.

## Paper Cuts

None recorded.

## Runtime Errors Observed

- The hosted beta.6 verifier reported `@compozy/extension-sdk beta points to 0.3.0-beta.5, want 0.3.0-beta.6`; later registry reads confirmed beta.6 without intervention (BUG-20260807-npm-dist-tag-readiness).

## Human Verifications Needed

- [ ] On the next release PR, confirm Setup Release Tools logs `Checksum verified` and `cosign signature verified` before GoReleaser starts (row #1).
- [ ] On the next hosted release, confirm stale npm dist-tags are retried until both packages converge, while terminal policy violations remain immediate failures (row #3).

## Decisions for a Human

None.

## Learnings

- GoReleaser Pro v2.17.1 and Compozy beta.5 use different Sigstore bundle generations; Cosign v3.1.3 verifies both with the existing identities and issuer.
- Rejecting an invalid release tag before downloads leaves no partial installation state.
- A successful npm publish can precede globally visible dist-tag metadata; post-publish acceptance must wait on the tag condition within a deadline.

## Final Status

**BLOCKED**

The release trust-chain checks, public-installer contract, live beta.5 install, and real Sigstore
bundle verification passed. The repository-wide lint blocker was repaired at its token-source root:
the deck minimum widths now use Tailwind's `--min-width-*` namespace, the generated CSS contains the
bare minimum-width utilities, and the Web test and production-build lanes pass. Hosted beta.6
proved the production Cosign fix and published the GitHub prerelease plus both npm packages, then
exposed an eventually consistent npm dist-tag read. The release-PR dry-run and the next hosted
release remain the two human verification boundaries.

Final make verify evidence: /Users/pedronauck/dev/qa-labs/compozy-release-cosign-v3-20260807-170358-478309-lab/qa-artifacts/qa/evidence/final-make-verify.log
