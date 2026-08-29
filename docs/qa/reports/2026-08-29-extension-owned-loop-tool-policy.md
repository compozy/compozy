# QA Run Report — 2026-08-29 — extension-owned-loop-tool-policy

- **Scope:** PR #503 extension-owned Loop action policy, exact-owner isolation, and restart hydration
- **Cadence tier:** targeted
- **Build:** `920083f0c2cbe9a11e497ab7c82b679f1c852d31` plus current review adjustments · **Environment:** isolated local daemon at `http://127.0.0.1:58217`; CLI/API/runtime required; browser unavailable and out of scope
- **Started:** 2026-08-29T16:32:44-03:00 · **Status:** passed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-loop-extension-owner-policy |

## Flows in Scope

- `J-loop-extension-actions` — Run an extension-owned Loop action without granting access to foreign extension tools (`../journeys/J-loop-extension-actions.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-extension-owner-policy | J-loop-extension-actions / LP-extension-owned-loop-tool-policy | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- Installed two independent external extensions from local build artifacts. Both were active and
  healthy after install and again after daemon restart.
- Confirmed `tools.policy.external_default = "disabled"` before execution and after restart.
- `own-flow` completed through the CLI; `ext__owner_one__echo` recorded
  `{"owner":"owner-one"}`. The HTTP API returned the same terminal Run and output.
- `foreign-flow` could not resolve `ext__owner_two__echo`; the task recorded
  `unknown_action_kind` and the daemon logged `tool "ext__owner_two__echo" not found`.
- Restarted the isolated daemon, reloaded the persisted Run with the same definition digest and
  output, then ran `own-flow` again successfully.

## What Was Fixed

No product findings required a fix. The first owner-one fixture build used Loop directories that did
not match `meta.name`; the fixture was corrected and rebuilt before behavioral execution.

## Paper Cuts

None recorded.

## Runtime Errors Observed

- Expected policy denial: `foreign-flow` emitted `unknown_action_kind` for the owner-two tool.
- Fixture-only setup error: the first owner-one install rejected mismatched Loop directory names;
  no partial extension state remained.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- The scenario initially referenced a missing journey and used a persona name absent from `personas.md`; planning now uses Bruno and a durable journey/charter pair.
- A completed persisted Run plus a fresh post-restart execution provides observable CLI/API evidence;
  the coordinator integration test separately exercises owner restoration on the execution context.

## Final Status

Behavioral verdict: **Pass**. Same-owner access remained exact, foreign-owner resolution stayed
denied, and restart did not require a global external-source grant. Evidence:
`/home/francisross/dev/qa-labs/compozy-extension-owned-loop-tool-policy-20260829-192832-898784-lab/qa-artifacts/qa/evidence/extension-owner-policy/result.json`.

Final verification:

- Local gate: `make gate` — Go lint and race-enabled scoped tests passed with zero issues.
- QA teardown: `/home/francisross/dev/qa-labs/compozy-extension-owned-loop-tool-policy-20260829-192832-898784-lab/qa-artifacts/qa/teardown.json` (`clean: true`).
