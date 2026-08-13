# QA Run Report — 2026-08-12 — issue-359-auto-update

- **Scope:** Issue #359 direct-binary extraction and release archive compatibility contract
- **Cadence tier:** targeted
- **Build:** 26f7b488 + working tree · **Environment:** isolated lab `/Users/pedronauck/dev/qa-labs/compozy-issue-359-auto-update-20260812-211235-947224-lab`; real GitHub beta.13 release; no provider or Web surface required
- **Started:** 2026-08-12T21:12:59Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Power User | desktop / wifi-fast / en-US | CH-beta-self-update-artifact-contract, CH-release-update-archive-contract |

## Flows in Scope

- `J-evaluate-compozy-beta` — update an isolated direct binary through the real beta channel (`../journeys/J-evaluate-compozy-beta.md`)
- `J-publish-compozy-beta` — reject a direct-update archive before release publication when the runtime cannot consume it (`../journeys/J-publish-compozy-beta.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-beta-self-update-artifact-contract | J-evaluate-compozy-beta / REL-beta-self-update | Dora | Feature Tour | Fixed | BUG-20260812-successful-update-recommends-retry | working-tree |
| 2 | CH-release-update-archive-contract | J-publish-compozy-beta / REL-release-archive-update-contract | Dora | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-beta-self-update-artifact-contract — Dora

- **Ran:** 2026-08-12T21:15:00Z → 2026-08-12T21:20:00Z (box respected: yes)
- **Findings:** The real beta.13 archive verified, extracted, and replaced the isolated beta.8 binary, but the successful JSON result retained the stale recommendation to run the update again (Trust-Damage).
- **Bugs filed/updated:** BUG-20260812-successful-update-recommends-retry
- **Scenarios settled:** REL-beta-self-update → pass after governed fix and retest
- **Paper cuts:** The unsupported `--version` flag was a dull CLI convention surprise; the documented `version` verb remained clear and functional.
- **Surprises:** The full signed beta.13 update completed in under three seconds after the release check cache was warm.
- **Suggested next charter:** Exercise the next candidate across another supported platform.

### CH-release-update-archive-contract — Dora

- **Ran:** 2026-08-12T21:20:00Z → 2026-08-12T21:27:00Z (box respected: yes)
- **Findings:** The exact `before_publish` Mage target accepted the real beta.13 archive and rejected a binary one MiB above policy with measured size and limit. No additional defect surfaced.
- **Bugs filed/updated:** None.
- **Scenarios settled:** REL-release-archive-update-contract → pass
- **Paper cuts:** None.
- **Surprises:** The 257 MiB rejection fixture compressed below 300 KiB, confirming that compressed size alone cannot protect extraction.
- **Suggested next charter:** Exercise the next candidate's four Darwin/Linux archives during release rehearsal.

## What Was Fixed

### BUG-20260812-successful-update-recommends-retry: Successful update still recommends another update

- **Symptom:** Successful structured output combined `status: updated` with a command to update again.
- **Root cause:** Terminal success reused the available-state record without clearing its recommendation.
- **Fix:** working-tree; both local and daemon-restart success paths clear the terminal recommendation.
- **Regression test:** `internal/cli/update_command_test.go` — red before, green after.
- **Retested:** `J-evaluate-compozy-beta` from a fresh isolated beta.8 candidate, plus the adjacent release archive journey.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Dora | J-evaluate-compozy-beta step 4 | "The update says it finished and immediately tells me to update again." | sharp | fixed (working-tree) |
| Dora | J-evaluate-compozy-beta step 4 | "I expected `--version`, but the help teaches the `version` verb." | dull | watching |

## Runtime Errors Observed

- Successful update output initially retained `Run compozy update`; fixed and absent from `qa/update-apply-final.json`.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Taxonomy plan: the two CLI/release flows cover journey, functional, error recovery, and producer-consumer continuity. Browser responsiveness, form input, locale, and accessibility are deliberate skips because this diff exposes no browser or input surface.
- The consumer walk uses the real GitHub archive, checksum catalog, and Sigstore bundle. Its only parity deviation is an isolated candidate executable and `COMPOZY_HOME`, required to protect the operator installation.
- The release walk invokes the exact configured Mage target without publishing, tagging, or changing a release.
- Edge probes covered check-only immutability, repeat current-state reads, operator-binary isolation, an unsupported file format, an extracted binary above policy, and a repeated real-archive hook run.

## Experiential Lens Pass

| Journey | Usability | Accessibility | Perceived performance | Compatibility | Error recoverability | Production parity |
|---|---|---|---|---|---|---|
| J-evaluate-compozy-beta | pass | pass | pass | pass | pass after fix | pass |
| J-publish-compozy-beta | pass | pass | pass | pass | pass | pass |

Compatibility is scoped to the live Darwin arm64 consumer walk; the producer hook is configured for
every Darwin/Linux archive. Browser, viewport, form, and locale checks are not applicable to these
text-only CLI and release-operator surfaces.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` — PASS; exact fingerprint and log recorded in the lab verification report and `.cache/gate/`.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 1 fixed and verified · Friction 0 · Cosmetic 0
- **Coverage:** 2/2 journeys walked; no skipped session rows
- **Verdict:** ready — both journeys passed, the only QA finding was fixed and re-walked, and no user-impact issue remains open.
