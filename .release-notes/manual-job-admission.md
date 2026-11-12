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
finishes in time, you get its final result. If the wait runs out, you get the running run and
the CLI still exits 0; the run is not canceled, and you can read its outcome later by run id.
Invalid input, errors before the run starts, and daemon or connection errors still exit
nonzero. `--timeout` without `--wait` is rejected.

Task and Loop targets return the same receipts as before, and extension host trigger calls
still wait for completion. No route, command, tool id, response field or stored data changes;
a retried manual run keeps its run id.
