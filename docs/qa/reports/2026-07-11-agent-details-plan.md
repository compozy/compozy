# Agent details QA plan — 2026-07-11

## Scope and cadence

Targeted cycle over exactly RT-028, RT-029, RT-069, and RT-074–RT-082. Task 07 flagged these rows; no other tracker row is retested. Journeys J-30–J-32 are executed through CH-049–CH-052.

## Exclusive scenario ownership

| Charter | Journey | Owned rows |
| --- | --- | --- |
| CH-049 | J-30 | RT-074, RT-075 |
| CH-050 | J-31 | RT-028, RT-076, RT-077, RT-082 |
| CH-051 | J-31 | RT-069, RT-078, RT-079, RT-080 |
| CH-052 | J-32 | RT-029, RT-081 |

Each in-scope row appears once. The union is the complete 12-row scope.

## Release-gate matrix

| Gate | Charter evidence |
| --- | --- |
| Cross-surface parity | CH-052 compares CLI, HTTP, UDS, and web-visible truth |
| Delete durability after restart/re-sync | CH-052 deletes, restarts, re-syncs, and re-reads |
| Duplicate fidelity including sidecars | CH-051 compares AGENT/SOUL/HEARTBEAT/MCP |
| No unbacked verb or metric | CH-049 partial-source tour + CH-052 structured-surface inventory |
| WCAG AA floor on three web surfaces | CH-049 responsive/keyboard pass + CH-050 accessibility tour + CH-051 dialog/settings focus pass |
| Approved design parity | CH-049 fleet, CH-050 detail, and CH-051 settings compare the three opendesign artifacts |

## Adversarial invariants

- CH-051 owns concurrent edit conflict, reload/reapply UX, dirty guard, permission denial, shadow disclosure, faithful duplication, and delete while a session is active.
- CH-052 owns `--yes` gating, API/UDS/CLI parity, durable restart/re-sync, un-shadow disclosure, and session/history survival.
- CH-049 owns the fleet partial-sessions-source failure without hiding definition truth.
- CH-050 owns authored-file CAS/history/wake eligibility, deep links, and agent-filtered session isolation.

## Deterministic lab contract

Task 09 creates a fresh lab with:

```bash
python3 .agents/skills/agh-qa-bootstrap/scripts/bootstrap-qa-env.py \
  --scenario agent-details-task-09 \
  --repo-root .
```

Use the manifest's `AGH_HOME`, `AGH_HTTP_PORT`, `AGH_UDS_PATH`, `TMUX_BRIDGE_SOCKET`, `AGH_WEB_API_PROXY_TARGET`, provider homes, browser policy, audit command, and teardown command. Seed dozens of agents; include a global/workspace same-name pair, an MCP/Soul/Heartbeat-bearing source, a duplicate target name, an active session, failed/done sessions, and an agent with diagnostics. Config writes are sequential. No parallel labs are planned; if execution introduces parallelism, each lab needs its own manifest-derived home, ports, and socket.

The run ends on every terminal path with the manifest `TEARDOWN_COMMAND`; completion cites `teardown.json` with `"clean": true`.

## Taxonomy sweep

- Functional: all 12 rows have one owner.
- Failure/recovery: partial sessions, 409, 403, declined confirmation, restart/re-sync.
- Persistence: URL filters, authored files, duplicate directory, deleted definition, surviving session history.
- Access/usability: keyboard/screen-reader, responsive widths, focusable denied actions, typed confirmation.
- Cross-surface: web/CLI/HTTP/UDS plus filesystem/catalog truth.
