# QA Run Report — 2026-08-20 — ACP tool update burst

- **Scope:** Coalesce redundant nonterminal ACP tool updates without losing canonical enrichments, terminal results, or prompt completion.
- **Cadence tier:** targeted
- **Build:** `f2147505` plus the `fix-acp-overflow` working tree · **Environment:** isolated CLI/runtime/provider lab at `http://127.0.0.1:45057`; deterministic ACP subprocess; no browser surface in scope
- **Started:** 2026-08-20T13:06:27Z · **Status:** PASS

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-acp-tool-update-burst |

## Flows in Scope

- `J-15` — An agent drives and reads sessions deterministically over CLI, HTTP, or UDS ([journey](../journeys/J-15-operate-session-via-cli-api.md)).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-acp-tool-update-burst | J-15 / RT-acp-tool-update-burst | Ada | Feature Tour | Pass | | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-acp-tool-update-burst — Ada

- **Entry:** Ada configured the `burst` provider and created `burst-operator` through the public CLI, then registered the isolated lab workspace and created session `sess-a3905975fcbee1c4`.
- **Disruption:** The ACP subprocess emitted one `tool_call`, 1,100 identical in-progress `tool_call_update` notifications, and one completed update for `tool-burst`.
- **Observed result:** `compozy session prompt -o jsonl` returned one tool input, one canonical `tool_call`, one terminal `tool_result`, and `finish`; it did not expose the repeated notifications or disconnect.
- **Recovery proof:** A second prompt on the same session completed with the same canonical sequence.
- **Independent readback:** `compozy session events --last 50 -o json` contained exactly one `tool_call`, one `tool_result`, and one `done` event per turn, all scoped to workspace `ws_9f783e456370bd67`.
- **Evidence:** `qa/logs/provider-readiness.json`, `qa/logs/first-prompt.jsonl`, `qa/logs/follow-up-prompt.jsonl`, and `qa/logs/session-events-summary.json` under the isolated lab.

## What Was Fixed

The ACP prompt state now remembers the latest canonical projection for each tool call. Repeated
nonterminal notifications with no new title, name, kind, input, or prechecked state are suppressed;
meaningful enrichments and terminal results still cross the bounded channel in order.

## Paper Cuts

The first `session new --cwd` resolved to the operator's existing default workspace. The QA pass did
not use that session: Ada registered the lab explicitly and created a second session with
`--workspace acp-burst-lab`, which reported the expected isolated workspace path.

## Runtime Errors Observed

None in the passing session. Both prompts exited successfully and the event readback contained no
provider disconnect event.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- The public daemon path proves the complete CLI, UDS, ACP subprocess, persistence, and follow-up-session behavior.
- The exact two-event prompt-buffer boundary is owned by the automated subprocess contract test; the public daemon does not expose that internal test seam as configuration.
- A deterministic provider is required here because an external model cannot guarantee 1,100 byte-for-byte repeated notifications in a fixed order.

## Verification Evidence

- Focused runtime: `CGO_ENABLED=1 go test -race ./internal/acp -count=1`
- Source-close gate record: `.cache/gate/full.json`
- Behavioral evidence: `/home/francisross/dev/qa-labs/compozy-acp-tool-update-burst-20260820-130528-159317-lab/qa-artifacts/qa/logs/session-events-summary.json`
- Strict evidence audit: `/home/francisross/dev/qa-labs/compozy-acp-tool-update-burst-20260820-130528-159317-lab/qa-artifacts/qa/qa-audit-report.json`
- Process teardown: `/home/francisross/dev/qa-labs/compozy-acp-tool-update-burst-20260820-130528-159317-lab/qa-artifacts/qa/teardown.json`

## Final Status

PASS — the repeated update burst completed twice through the public session surface with one
canonical call/result pair per turn, no duplicate durable events, and no provider disconnect. The
provider was deterministic by design; all CompozyOS runtime and transport layers were production
paths.
