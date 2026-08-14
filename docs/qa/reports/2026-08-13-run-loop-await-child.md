# QA Run Report — 2026-08-13 — run-loop-await-child

- **Scope:** Preserve an awaited child Loop's identity and blocking state across task completion and daemon restart.
- **Cadence tier:** targeted
- **Build:** `compozy v0.3.0-beta.15-15-gea7c17e4-dirty` · **Environment:** fresh isolated QA labs using the branch-built daemon; the primary ordering walk has no extension, agent, provider, or mock dependency, followed by a separate Batuta compatibility canary
- **Started:** 2026-08-13T17:16:07-03:00 · **Status:** BLOCKED (upstream gate baseline)

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-await-child-loop-restart |

## Flows in Scope

- `J-await-child-loop` — compose durable child work without skipping or duplicating it after restart (`../journeys/J-await-child-loop.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-await-child-loop-restart | J-await-child-loop / LP-run-loop-await-child-ordering | Bruno | Interrupt Tour | Pass | [#386](https://github.com/compozy/compozy/issues/386) | this PR |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-await-child-loop-restart — Bruno

- **Ran:** 2026-08-13T17:18:48-03:00 → 2026-08-13T17:20:23-03:00 (box respected: yes)
- **Findings:** The parent stayed `running`; `first_child` stayed `awaiting_child`; `second_child` stayed `pending`; and exactly one child id survived the daemon restart. Releasing the first wait moved only that child to `done` before the second child was created. Releasing the second wait then moved the parent and both children to `done`. CLI/UDS and HTTP terminal reads matched.
- **Bugs filed/updated:** Upstream issue [#386](https://github.com/compozy/compozy/issues/386) documents the fixed defect; no new runtime bug was found.
- **Scenarios settled:** LP-run-loop-await-child-ordering → pass.
- **Paper cuts:** A timed wait requires `--payload '{}'` to select the manual-wait resume path; omitting it attempts a paused-node transition and returns `node_not_paused` (dull, outside this fix).
- **Surprises:** The CLI enters through UDS, so an authored reproduction intended for CLI use must declare `start: uds`; the generic design reproduction now declares manual, HTTP, and UDS starts explicitly.
- **Suggested next charter:** Re-run this charter after merge as a smoke canary against the packaged release binary.

## What Was Fixed

### Issue #386: awaited run-loop output lost its child identity

- **Symptom:** A parent could report `done` and start later nodes while awaited child Loops were still live.
- **Root cause:** Completed mechanical task output was collapsed to generic success before the coordinator reconstructed the reserved `run-loop` await result.
- **Fix:** Decode and validate the persisted result, restore `awaiting_child` plus the exact child id, and reuse the existing child-terminal refresh path; fail closed on malformed or foreign child identities.
- **Regression test:** `internal/loop/coordinator_test.go` failed before the fix because the restored parent did not yield while its child was live; focused unit and restart E2E now pass.
- **Retested:** J-await-child-loop through public CLI/UDS, HTTP, daemon restart, and durable wait resume in a fresh isolated lab.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Bruno | J-await-child-loop step 3 | "I need to know that an empty payload selects wait-resume; otherwise the error talks about a paused node." | dull | watching |

## Runtime Errors Observed

- `node_not_paused` was returned when `loop node resume` omitted `--payload`; adding the documented payload selected manual-wait resume and succeeded. This did not affect the runtime-ordering verdict.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Public CLI starts are transported over UDS; generic reproduction definitions must allow the actual ingress kind.
- Persisted child identity is independently observable through CLI/UDS and HTTP, so no database or internal test seam is needed for the production diagnosis.

## Compozy Impact Audit

- **Runtime contract:** Awaited `run-loop` task results now restore `awaiting_child` plus the exact persisted child id before the coordinator considers dependents. Existing detach behavior remains immediate success.
- **Terminal mapping:** `done` and `no-op` children settle the parent node as succeeded; failed and canceled children settle it as failed. Live children continue yielding through the existing coordinator path.
- **Recovery:** Restart E2E proves the first child id survives daemon restart, no duplicate child is submitted, and the second child remains pending until the first is terminal.
- **Workspace and ownership fence:** Unit coverage rejects a child from another workspace or another parent and clears the untrusted child id instead of normalizing or retaining it.
- **Malformed durable state:** Empty, malformed, incomplete, padded, or contradictory awaited results fail closed with the stable existing `action_schema_invalid` reason and operator-safe diagnostic.
- **Missing authored owner:** A completed action whose graph node is absent now fails closed as `action_schema_invalid` instead of being promoted to success.
- **Public surfaces:** The isolated walk compared CLI/UDS and HTTP before and after restart and again at terminal state. Native-tool IDs, HTTP/UDS schemas, hooks, config, extension formats, and Web behavior are unchanged.
- **Generated contracts:** No public enum or schema shape changes in this fix. `make codegen-check` remains part of the final gate; no generated TypeScript update is expected.
- **Documentation:** Issue #386 and the checked-in design use a standalone authored-Loop reproduction with the actual manual/HTTP/UDS ingress allowlist. The official Loop skill and site guardrails describe ordering, restart, and terminal mapping. Batuta remains discovery evidence only.

## Prior Review Learnings Applied

- Regression fixtures assert the precondition that made the old implementation fail, not only the final success state.
- New Go test cases use named `Should...` subtests, parallel execution, specific stable diagnostics, and outcome assertions rather than call-count-only checks.
- The E2E waits on durable observable state instead of a fixed quiet window and verifies both children terminal, not only the parent.
- Production logic lives in a focused file below the 500-line cap; new helpers have concise Go doc comments and share one failure-construction path.
- Error handling distinguishes authored detach, valid await, malformed state, and ownership violations without permissive fallback or silent normalization.

## Final Status

- **Focused verification:** 21 new regression cases passed under race; the complete `internal/loop` package passed 807 tests under race; focused daemon restart E2E passed; canonical `test-e2e-runtime` passed daemon, HTTP, UDS, harness, and CLI lanes.
- **Coverage:** `internal/loop` statement coverage is 78.1%.
- **Build and Batuta compatibility:** `make build` passed; the branch binary has SHA-256 `8875c04907b9e40d54b8e9cc693361c10a1c8b1439ec4ddac27c28dfcdf316ad`. A separately isolated Batuta canary opened the exact `auto_commit=false` gate, preserved `todo 1.0.0`, created one task, ran one `batuta-deliver`, and finished the `implement-tasks` and `review-and-fix` children `done`. An intentional daemon crash while the parent awaited review recovered the same child run and recorded one requeue with no duplicate child. The origin agent session was terminated by that deliberate crash, so its terminal conversational wake was not part of the recovery verdict.
- **Exit gate (full automated suite):** BLOCKED by failures reproduced on clean `upstream/main` at `a0761a0f`, outside this branch: one CLI config-validation expectation, one uppercase agent fixture rejected by the accepted agent-name contract, and release tag tests affected by the local signed-tag environment. The relevant changed packages and lint lanes passed. Evidence: `/home/franciscpd/dev/qa-labs/compozy-run-loop-await-child-20260813-201607-890089-lab/qa-artifacts/qa/final-make-verify.log`
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 1 · Friction 0 · Cosmetic 0
- **Journey coverage:** 1/1 journeys walked; browser and provider were intentionally out of scope for this targeted non-agent runtime journey.
- **Verdict:** BLOCKED — the isolated behavior-first walk, automated recovery coverage, and Batuta compatibility canary pass the changed behavior, but the repository-wide exit gate cannot be reported green until the independently reproduced upstream baseline failures are resolved.
