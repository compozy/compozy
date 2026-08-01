# QA Run Report — 2026-08-01 — release-note-signal

- **Scope:** Prevent squash-generated conventional titles after a breaking footer from leaking into release changelogs
- **Cadence tier:** targeted
- **Build:** `60657996` plus the working-tree git-cliff preprocessor · **Environment:** local production release CLI with the pinned releasepr v0.0.25 and git-cliff v2.12.0; public GitHub verification pending deployment
- **Started:** 2026-08-01T14:11:03-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Power User | desktop / wifi-fast / en-US | CH-release-note-signal |

## Flows in Scope

- `J-approve-compozy-beta-candidate` — A release administrator can prove which commit, version, channel, and policy would ship without publishing (`../journeys/J-approve-compozy-beta-candidate.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-release-note-signal | J-approve-compozy-beta-candidate / REL-release-note-signal | Dora | Feature Tour | Blocked (needs human verify) | Local candidate clean; public PR #272 awaits regeneration from `main` | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-release-note-signal — Dora

- **Ran:** 2026-08-01T14:11:42-03:00 → 2026-08-01T14:14:09-03:00 (box respected: yes)
- **Entry:** `releasepr release-body --tag v0.3.0 --range v0.2.15..HEAD`
- **Findings:**
  - The pinned release CLI retained refactors, features, fixes, the authored release notes, and the MCP breaking summary without promoting the four squash-generated `fix`/`test` titles.
  - The same output remained clean after the production `oxfmt --stdin-filepath RELEASE_BODY.md` formatting step.
  - An independent `git-cliff --context` read returned only the two-line MCP breaking summary for commit `7f3fb641`.
  - Every published GitHub Release from `v0.1.0` through `v0.3.0-beta.2` contained zero loose `* fix:` or `* test:` entries.
  - Open PR #272 still contains the four old loose entries because the fix has not reached `main`; this is a deployment boundary, not evidence against the working-tree candidate.
- **Bugs filed/updated:** none; this run verifies the user-reported defect directly.
- **Scenarios settled:** `REL-release-note-signal → blocked-verify`
- **Paper cuts:** none.
- **Surprises:** the defect affected both rendered Markdown and the JSON context before the centralized preprocessor; fixing only the template would have left the site receipt exposed.
- **Suggested next charter:** re-run `CH-release-note-signal` after the release workflow regenerates PR #272.

## What Was Fixed

### Reported release-body list leakage

- **Symptom:** Four conventional child-commit titles appeared as loose Markdown bullets after the MCP breaking summary in release PR #272.
- **Root cause:** GitHub appended later squash-commit titles after a `BREAKING CHANGE` footer, and the Conventional Commit parser absorbed that tail into `breaking_description`.
- **Fix:** `cliff.toml` removes only a trailing GitHub-style list of conventional commit titles before `git-cliff` parses the footer.
- **Regression proof:** the real commit `7f3fb641` leaked four bullets before the change and none after it; a synthetic authored bullet (`* Operators must migrate explicitly.`) remains preserved.
- **Retested:** the pinned `releasepr release-body` path, `oxfmt` output, `git-cliff --context`, open PR #272, and all published GitHub Releases.

## Paper Cuts

None observed.

## Runtime Errors Observed

None observed. The release CLI emitted the expected no-token informational warning and performed no GitHub mutation.

## Human Verifications Needed

- [ ] After the fix reaches `main`, wait for the release workflow to regenerate PR #272, reopen the PR through a fresh GitHub read, and confirm the four `* fix:`/`* test:` entries are absent while the MCP breaking summary remains.

## Decisions for a Human

None.

## Learnings

- Normalize externally generated squash messages once, before every release consumer, rather than independently patching Markdown and site receipts.
- A release artifact can pass locally while its public candidate remains stale; the tracker must keep those claims separate.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` runs after this final report mutation; authoritative content-keyed result is recorded by `make gate-status` and cited in the task completion.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1 journey walked / 1 in scope; local production CLI and published history verified, open release PR verification blocked on deployment.
- **Verdict:** ready with blocked items — the implementation is locally verified; PR #272 must be re-read after regeneration from `main` before candidate approval.
