# QA Run Report — 2026-08-14 — CodeRabbit git-rebase helper

- **Scope:** Targeted CLI walk for the bundled git-rebase analyzer and validator after CodeRabbit remediation.
- **Cadence tier:** targeted
- **Build:** `0ead9baf` · **Environment:** isolated CLI-only QA lab; no daemon or provider required
- **Started:** 2026-08-15T00:15:48Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-git-rebase-helper-safety |

## Flows in Scope

- `J-offer-runnable-capabilities` — an agent receives a runnable, truthful bundled capability (`../journeys/J-offer-runnable-capabilities.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-git-rebase-helper-safety | J-offer-runnable-capabilities / ET-git-rebase-helper-safety | Ada | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-git-rebase-helper-safety — Ada

- **Ran:** 2026-08-15T00:15:48Z → 2026-08-15T00:21:17Z (box respected: yes)
- **Findings:** No product defect remained. The analyzer preserved `file with spaces.txt` as one shell-escaped path and moved from one unresolved conflict to zero after resolution. The validator rejected conflict markers even after the file was staged, then passed when the markers were removed.
- **Bugs filed/updated:** None.
- **Scenarios settled:** ET-git-rebase-helper-safety → pass.
- **Paper cuts:** None.
- **Surprises:** A staged resolved path must remain in the marker scan even though Git no longer labels it unmerged; the final implementation scans only changed paths against `HEAD`.
- **Suggested next charter:** Re-run this charter when the public git-rebase helper changes again.

## What Was Fixed

The implementation fix predates this QA session and is covered by the CodeRabbit remediation workstream.

## Paper Cuts

None.

## Runtime Errors Observed

None.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- NUL-delimited path discovery and array length are both necessary: fixing only iteration still leaves empty and spaced-path counts unreliable.
- Marker scanning should follow the changed-path set, not the whole repository and not only Git's unmerged set.

## Final Status

- **Exit gate (targeted automated suite):** `shellcheck` on both delivery copies plus the isolated behavioral replay — PASS; workstream gates run after this report is finalized
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1 of 1 journeys walked; no skips
- **Verdict:** ready — the changed helper behavior is verified through its public CLI surface and an independent Git read path.
