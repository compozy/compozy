# QA Run Report — 2026-08-03 — session event owner isolation

- **Scope:** PM01 hard cut: bind every per-session `events.db` to one immutable session/workspace owner and refuse foreign or ownerless files before migration or mutation.
- **Cadence tier:** targeted
- **Build:** `741d3563` + working tree · **Environment:** fresh isolated lab at `http://127.0.0.1:52805`, CLI/HTTP/UDS public surfaces, no provider or Web dependency.
- **Started:** 2026-08-03T05:00:43-03:00 · **Status:** complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-session-event-owner-refusal |

## Flows in Scope

- `J-operate-daemon-schema` — start and inspect the daemon schema safely, including a foreign-owned per-session event store (`../journeys/J-operate-daemon-schema.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-event-owner-refusal | J-operate-daemon-schema / RT-session-event-owner-isolation | Bruno | Garbage Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

Bruno started with two public-history canaries in separate registered workspaces. With the daemon
stopped, the run preserved both complete session directories, substituted beta's intact SQLite
family under alpha, and restarted. Boot repair, CLI history/events, HTTP history, and direct UDS
history all refused alpha because the persisted owner named beta. The daemon stayed available and
beta remained readable. After the refused reads, all three foreign SQLite-family hashes were exactly
unchanged. Restoring alpha's complete saved directory restored CLI, HTTP, and UDS reads and returned
only alpha-owned events.

## What Was Fixed

No QA finding has required a fix.

## Paper Cuts

None.

## Runtime Errors Observed

The expected owner-mismatch path appeared in boot repair and CLI/UDS diagnostics. HTTP intentionally
returned a generic `500` body while the daemon logged the bounded internal cause. No unexpected
runtime error occurred.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Ownership is enforced at boot repair and on-demand read paths, so one foreign session store does
  not take down the daemon-global database or a correctly owned sibling session.
- Refusal is non-mutating across the supplied `events.db`, WAL, and SHM files. Recovery requires the
  matching complete session directory; editing owner or migration rows is neither required nor
  supported.
- The targeted behavior needs neither a provider session nor Web rendering. The generic strict
  auditor therefore reports its wider release-profile minimums separately from this scenario's
  focused pass.

## Lab Evidence

- Bootstrap manifest: `/home/pedronauck/dev/qa-labs/compozy-session-event-owner-isolation-20260803-080043-333009-lab/qa-artifacts/qa/bootstrap-manifest.json`
- Scenario contract: `/home/pedronauck/dev/qa-labs/compozy-session-event-owner-isolation-20260803-080043-333009-lab/qa-artifacts/qa/scenario-contract.json`
- Behavioral charter: `/home/pedronauck/dev/qa-labs/compozy-session-event-owner-isolation-20260803-080043-333009-lab/qa-artifacts/qa/behavioral-scenario-charter.yaml`
- Public walk and byte hashes: `/home/pedronauck/dev/qa-labs/compozy-session-event-owner-isolation-20260803-080043-333009-lab/qa-artifacts/qa/evidence/session-owner/session-owner-walk.md`
- Journey log: `/home/pedronauck/dev/qa-labs/compozy-session-event-owner-isolation-20260803-080043-333009-lab/qa-artifacts/qa/journey-log.jsonl`
- Strict audit: `/home/pedronauck/dev/qa-labs/compozy-session-event-owner-isolation-20260803-080043-333009-lab/qa-artifacts/qa/qa-audit-report.json`
- Teardown: `/home/pedronauck/dev/qa-labs/compozy-session-event-owner-isolation-20260803-080043-333009-lab/qa-artifacts/qa/teardown.json` (`clean=true`, zero survivors)

## Final Status

- **Exit gate (full automated suite):** intentionally pending; the modernization workstream runs `make gate-full` once after its final mutation.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 targeted journey session walked across CLI, HTTP, direct UDS, daemon boot repair, two workspaces, byte-integrity refusal, sibling canary, and complete-directory recovery.
- **Strict generic profile:** blocked by C4/C5/C6/C7/C8/C9/C10/C11 and the intentionally deferred C14 full gate. Those checks require a release-scale multi-actor/provider/Web/task/disruption/artifact exercise outside this focused persistence charter; the contract was not weakened and no evidence was invented.
- **Verdict:** PASS for `RT-session-event-owner-isolation`. The foreign store was refused without SQLite-family mutation or cross-workspace disclosure, the sibling stayed available, and matching-directory restore recovered the target.
