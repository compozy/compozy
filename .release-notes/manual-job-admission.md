---
title: Manual job triggers return the admitted run immediately
type: fix
breaking: true
---

Manual job triggers now return the run id and `running` status after the agent prompt
is accepted. Long turns no longer make the CLI report a 30-second request timeout
while an admitted run continues without returning its id. The daemon continues to
own execution, retries, final state and session cleanup.

## Migration notes

Issue #1568 explicitly requests admission as the default and completion waiting as
an opt-in. Scripts that relied on the former blocking default must request waiting:

- CLI: `compozy automation jobs trigger <id> --wait --timeout 5m -o json`.
- HTTP/UDS: `POST /api/automation/jobs/{id}/trigger?wait=true&timeout=5m`.
- Native tool `compozy__automation_jobs_trigger`: `{job_id:"<id>",wait:true,timeout:"5m"}`.

The completion wait defaults to one minute and accepts a positive duration up to
24 hours. On expiry it returns the admitted running receipt without canceling the
run. A client disconnect before prompt acceptance cancels unaccepted work; after
admission it ends only the wait. Task and Loop targets retain delegated receipts;
extension host trigger calls retain their synchronous contract.

No route, verb, tool id, response field or persisted shape is removed. Existing run
ids and history remain intact. Manual triggers retain one run id across
backoff retries; the attempt and status update on that same run.
