---
id: RT-secret-redaction-boundary
area: RT
title: Redact planted secrets before storage and streaming
persona: Dora
journey: J-keep-secrets-contained
expected: With redaction heuristics enabled at daemon boot, a planted provider-shaped secret appears only as the canonical redaction marker in runtime logs, SSE, session history, the global event ledger, and the session events database. Correlation IDs and hashes remain intact. Disabling the heuristic and restarting leaves exact claim-token, secret-reference, and registered-secret protections active.
entry_points: General Settings; agh config get/set; agh__config_get/set; daemon logs; session SSE/history; global and session event stores
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-013; NB-bridge-tool-progress; LP-050
---

Start an isolated daemon with the default redaction setting and emit one unique provider-shaped
fixture secret through an ACP assistant response and tool input. Confirm that logs, SSE, history,
the global event ledger, and the session `events.db` contain the redaction marker and no raw secret.
Confirm the same records retain their session, run, and hash correlation values.

Set `redact.enabled` to `false` through a public config surface and confirm the mutation reports a
required daemon restart. Restart, emit an exact claim token and a registered secret, and confirm
both remain redacted even though the additive heuristic is disabled.

QA impact 2026-07-15: new default-on, process-snapshotted cross-surface secret redaction behavior.
Planning flag only; no QA session ran in this implementation slice.

Phase C planning 2026-07-19: persona normalized to Dora and linked to J-keep-secrets-contained;
settles US-009 (G2, ADR-005, N-402).

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Harness grep over captured logs, SSE stream, and dumps of `runtime.db` AND `events.db` → zero raw
  hits for the planted fixture secret.
- A log record carrying an intact `claim_token_hash` and session/run ids (envelope survival).
- The restart-required response for the `redact.enabled` mutation and the post-restart run proving
  exact claim-token/registered-secret protection stays active.
- Site build/link check confirming `SECURITY.md` renders and states `BackendLocal` is not isolation.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-009-secrets-never-surface
