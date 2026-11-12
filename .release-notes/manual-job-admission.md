---
title: Triggering a job returns its run right away
type: fix
breaking: true
---

`compozy automation jobs trigger <id>` now returns as soon as the job's agent accepts the
prompt, with the run id and `running` status, and exits 0. The run keeps going in the daemon,
which still owns retries, the final result and session cleanup. Before, the command waited for
the whole agent turn, so a turn longer than 30 seconds made the CLI report a request timeout
while the run kept going, and you never got its run id.

The HTTP/UDS trigger route and the `compozy__automation_jobs_trigger` native tool return the
running run the same way.

## Migration notes

A successful trigger now confirms that the run started, not that it finished. Scripts that
treated it as completion must ask to wait or follow the returned run id:

- CLI: `compozy automation jobs trigger <id> --wait --timeout 5m -o json`
- HTTP/UDS: `POST /api/automation/jobs/{id}/trigger?wait=true&timeout=5m`
- Native tool: `{"job_id":"<id>","wait":true,"timeout":"5m"}`

The wait defaults to one minute and accepts any positive duration up to 24 hours. If the run
finishes in time, you get its final result; a run that fails within the wait returns its error
and exits nonzero, as before. If the wait runs out, you get the running run and the CLI still
exits 0; the run is not canceled, and you can read its outcome later by run id. Invalid input,
errors before the run starts, and daemon or connection errors still exit nonzero. A timeout
without wait is rejected (`--timeout` requires `--wait`).

Task and Loop targets return their delegated receipts immediately, as before, even with
`--wait`. Extension host trigger calls still wait for completion. The `wait` and `timeout`
inputs are additive: no route, command, tool id, response field or stored data is removed or
renamed, and a retried manual run keeps its run id.
