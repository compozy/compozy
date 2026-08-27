---
id: RT-call-payload-sanitize-sweep
area: RT
title: Sanitize call and message payloads before every downstream stage
persona: Dora
journey: J-contain-and-audit-delegation
expected: A planted claim-token-shaped value survives nowhere — not in the stored payload, projection, daemon log, SSE, canonical event, hook payload or repair prompt — while correlation ids and hashes stay intact and validator errors read verbatim from the sanitized output.
entry_points: compozy call reviewer "Inspect token ghp_fixture_secret"; compozy__call_return with {"result":{"token":"ghp_fixture_secret"}}; compozy message send ses_01JBD8G2MZTX "token ghp_fixture_secret"; compozy call show call_01JBD8G2K7Q9 and compozy call result call_01JBD8G2K7Q9; compozy logs; HTTP and UDS GET /api/logs; extension hook payloads; the repair prompt shown to the child
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-secret-redaction-boundary; RT-compozy-claim-token-redaction; ET-call-hooks-host-api-reads
---

This scenario is about **ordering**, not about which sinks redact. Sanitization is the first
admission stage: classification and contract-preserving redaction run on the raw payload bytes
before schema validation, before validator-error construction, before hook dispatch, before the
repair prompt is rendered, before any event is emitted and before anything is persisted. Every
later stage therefore only ever sees sanitized or hash-form data.

Plant a claim-token-shaped value three ways — in a call prompt, in a returned result, and in a
message body — then sweep every downstream sink for it: the stored payload, the public projection,
daemon logs, SSE, the canonical event stream, an installed extension's hook payload, and the repair
prompt the child is shown after a contract violation. Only the redaction marker may appear, and
correlation ids and hashes must survive intact so the trail is still followable.

The two traps worth aiming at directly: make the payload fail validation so the **validator error
text** is constructed from it — "errors verbatim" must mean verbatim from the sanitized output, not
from the raw bytes — and construct a payload where redaction cannot preserve contract validity, which
must fail with a fixed typed error naming the offending paths but never their values.

Distinct from `RT-secret-redaction-boundary` (which owns where planted secrets may surface across
durable stores and streams) and `RT-compozy-claim-token-redaction` (which owns the lease boundary):
those two ask *where*, this one asks *when*.
