---
id: ET-tool-result-artifact-recovery
area: ET
title: Recover one oversized tool result across public surfaces
persona: Rafa
journey: J-14
expected: An oversized post-hook redacted tool result keeps a truthful preview, opens as exact ordered bytes in Web, native tool, CLI, HTTP, and UDS, remains isolated to its workspace, survives daemon restart until deterministic retention removes it, and preserves a bounded partial result if persistence fails.
entry_points: Web session tool-result card; agh__tool_artifact_read; agh tool artifact read; GET /api/workspaces/:workspace_id/tool-artifacts/:artifact_id
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Invoke a deterministic tool whose final post-hook, redacted result exceeds its effective result
budget and includes a multi-byte character across the 64 KiB page boundary. Confirm the transcript
keeps the bounded preview and one opaque artifact URI. Open every page in Web, then repeat through
the native reader, CLI, HTTP, and UDS; concatenate bytes in offset order and require the same
canonical JSON envelope on every surface.

Restart the daemon and read the artifact again. Confirm another workspace receives the same
not-found shape as an expired reference. Exercise count, byte, and age retention independently,
then inject one persistence failure and require a typed error with the bounded partial result and no
fabricated durable URI. Confirm the Web card preserves the preview through loading, not-found, and
retry states.

QA impact 2026-07-19: new oversized tool-result retention, paging, management surfaces, and Web
viewer. Planning flag only; no QA session ran in this implementation slice.

Phase C planning 2026-07-19: settles defect D6 (TechSpec §3.1 tool-result offload) in the coverage
map; also proves US-003 EC-4 (artifact page-back on degraded resume).

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Byte-identical page-back reads (offset-ordered concatenation) across Web, native, CLI, HTTP, and
  UDS for the oversized fixture, including the multi-byte page boundary.
- Post-restart read success, the cross-workspace not-found probe, and the three retention paths.
- The injected persistence failure returning a typed error with the bounded partial result and no
  fabricated durable URI.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-002-oversized-tool-results
