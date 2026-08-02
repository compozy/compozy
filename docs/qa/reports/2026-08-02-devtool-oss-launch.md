# QA Run Report — 2026-08-02 — devtool-oss-launch

- **Scope:** One-kickoff autonomous collaboration across the Helix CLI release, docs, benchmark, DevRel, and community workspaces, plus the provider-auth hard-cut surfaces exercised in the same isolated lab.
- **Cadence tier:** full
- **Build:** `741d3563` · **Environment:** isolated Compozy daemon at `127.0.0.1:40279`, real local Codex ACP sessions, five playbook workspaces, isolated `COMPOZY_HOME=/tmp/compozyqa-5c85c0f2b17e/runtime`
- **Started:** 2026-08-03T01:18:17Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power operator | Linux desktop, isolated localhost daemon, `pt-BR` operator context | `devtool-oss-launch` behavioral charter |
| Mateo Rivera | Technical founder/CEO | Real Codex provider session, five workspace team, public Network channels | `sess-dda3ada510e1fc02` |

## Flows in Scope

- `J-one-kickoff-collaboration` — one founder kickoff activates the registered team, produces the declared release artifacts, absorbs disruptions, completes reviews, and ends in an evidence-backed ship or hold decision (`../journeys/J-one-kickoff-collaboration.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | `devtool-oss-launch` behavioral charter | `J-one-kickoff-collaboration` / `RT-073` | Bruno / Mateo Rivera | Scenario playbook | Blocked (human decision) | `BUG-0028`; `BUG-20260729-agent-knowledge-refresh-missed` | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### devtool-oss-launch behavioral charter — Bruno / Mateo Rivera

- **Ran:** 2026-08-03T01:36:04Z → 2026-08-03T02:06:04Z (box respected: yes)
- **Findings:**
  - **Blocks-Completion:** only 2 of 11 declared Tasks reached `completed`; 9 remained `ready` or unsettled when the fixed window closed. The runtime produced useful partial work, but the declared autonomous journey did not finish.
  - **Trust-Damage:** the benchmark owner detected the visible 22% cold-start regression 17m06s after the corrected workspace knowledge write, missing the required five-minute recovery window by more than twelve minutes. The eventual measurement and performance-review verdict were correct.
  - The breaking flag rename and signing-key failure were both surfaced publicly and converted into `HOLD` decisions without a second operator prompt or signing retry.
  - Three complete public review cycles were evidenced: docs landing approval, benchmark regression verdict, and launch-component artifact approval. No resolved inter-agent disagreement was evidenced.
- **Bugs filed/updated:** `BUG-0028`; `BUG-20260729-agent-knowledge-refresh-missed`
- **Scenarios settled:** `RT-073 → fail`
- **Paper cuts:** the Go canary stub reached 594 production lines in the generated scenario workspace, and two runtime review records remained `in_review`; both increase review and completion friction even though the files themselves parse.
- **Surprises:** observer-authored indexing kept all five surfaces visible and prevented a false stall diagnosis, but the runtime still does not own the journey-log projection tracked by `BUG-20260719-autonomous-progress-unobservable`.
- **Suggested next charter:** repeat the same playbook after the scheduler/worker completion boundary and knowledge-refresh contract are fixed, preserving the single-kickoff/no-follow-up constraint.

## What Was Fixed

No production fix was applied during this session. The run is evidence for the broader Go modernization workstream; missing artifacts were not authored by the observer.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Bruno | `J-one-kickoff-collaboration`, benchmark disruption | "The release stayed safe, but I could not trust the worker to notice changed workspace knowledge promptly." | sharp | Re-found `BUG-20260729-agent-knowledge-refresh-missed` |
| Mateo Rivera | `J-one-kickoff-collaboration`, cutover close | "Useful artifacts existed, but most declared Tasks were still open when the release window ended." | sharp | Re-found `BUG-0028` |

## Runtime Errors Observed

- The forced signing failure ended `run-540c90fd47f6af18` as `operator_forced`; the release owner surfaced it to `release-room`, made no silent retry, and retained `HOLD`. Evidence: `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/disruption-release-signing-result.json`.
- The first benchmark seed write targeted the lab seed source instead of the already-materialized agent workspace. The observer corrected the target at 2026-08-03T01:39:21Z; the recovery SLA is measured from that corrected write. Evidence: `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/disruption-bench-regression-result.json`.
- `browser-use` required a manual Chrome remote-debug approval unavailable in the isolated run; the mandated `agent-browser` fallback captured the live Task catalog successfully. Evidence: `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/web-tasks-runtime.png`.

## Human Verifications Needed

None. The incomplete journey is a product/runtime failure, not a leg that requires a person to simulate.

## Decisions for a Human

### One-kickoff work remains incomplete at the fixed window (`BUG-0028`)

- What's broken: the runtime started every declared run and produced several valid artifacts, but only two Tasks completed and required Python/TypeScript/spec deliverables were absent at window close.
- Why not auto-fixed: the cause spans scheduler admission, worker task progression, review settlement, and agent behavior; it exceeds a bounded QA fix.
- Options: 1. Diagnose the unfinished run state machines and worker continuation contract before replaying the same playbook. 2. Increase the scenario window, which would conceal rather than explain why a 60-minute business cutover cannot converge in the current 30-minute acceptance box.
- Recommendation: choose option 1 and preserve the existing time box.

### Workspace knowledge changes are observed too late (`BUG-20260729-agent-knowledge-refresh-missed`)

- What's broken: an active benchmark worker did not observe the materialized knowledge change inside five minutes and only escalated after 17m06s.
- Why not auto-fixed: the missing contract may be knowledge invalidation, wake-context assembly, or worker re-entry; production tracing is required before changing behavior.
- Options: 1. Add a runtime-owned knowledge revision signal to eligible wake context. 2. Depend on worker polling/re-reading conventions.
- Recommendation: choose option 1 so correctness does not depend on prompt discipline.

## Learnings

- The scheduler barrier and native claim path now activate all declared runs; the earlier zero-run failure mode did not recur.
- Public Network coordination is strong enough to surface signing and compatibility blockers without observer prompting.
- A disruption is not successful merely because it is eventually noticed: recovery latency is part of the contract.
- Runtime-owned journey-log progress remains a separate open observability gap; observer-authored evidence is useful indexing, not a production event stream.

## Evidence Index

- Smoke evidence: `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/api-runtime-snapshot.json`
- Behavioral evidence: `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/observation-summary.json`
- Final runtime snapshot: `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/final-runtime-snapshot.json`
- Journey log: `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/journey-log.jsonl`
- Provider proof: `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/provider-attempt.json`
- Web capture: `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/web-tasks-runtime.png`

## Final Status

- **Exit gate (full automated suite):** NOT RUN — the broader Go modernization workstream is still open, source is not frozen, and the repository rule permits the single `make gate-full` only after the final mutation. This is a blocking C14 condition for the strict scenario audit, not green evidence.
- **Strict evidence audit:** FAIL — C14 has no final full-gate evidence; C16 found only 1/2 required Python scripts and 0/1 required TypeScript test; C17 found 0/1 resolved disagreements. The earlier evidence-index gaps (C7/C8) and observer-authored forbidden wording (C15) were corrected before this final audit state. Evidence: `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/qa-audit-report.json`.
- **Issues by user impact:** Blocks-Completion 1 · Data-Loss 0 · Trust-Damage 1 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 in-scope one-kickoff journey walked across CLI, HTTP API, Web, runtime, native-tool, settings, and real provider surfaces; the expected autonomous end state was not reached.
- **Verdict:** not ready — the scenario is **BLOCKED** from PASS by incomplete declared work, late knowledge-disruption recovery, absence of a resolved disagreement, and the intentionally deferred full gate.
