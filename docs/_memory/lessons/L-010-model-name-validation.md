# L-010 — Non-existent model name silently breaks the entire batch

**Class:** Workflow / CI
**Date discovered:** 2026-04-24 (session-driver-override review batch at 02:17 UTC)
**Evidence sources:** local_runs + run.json

## Context

`reviews-session-driver-override-79f260-round-000-20260424-021709` failed with `ACP error -32603: stream disconnected before completion: The model 'gpt-5.5' does not exist`. The codex driver had been configured with an invalid model name in `run.json`. The upstream ACP API silently passed the misconfigured request to the provider, which rejected it.

Net result: a 13-issue review batch was wasted before any work could begin. The user retried with the correct model (`gpt-5.4`) within the hour and the batch succeeded.

## Root cause

Compozy launches a batch run without first validating the configured model against the IDE's actual model list. By the time the model name reaches the provider, the entire orchestration scaffolding has been allocated, the prompt has been rendered, and the failure surfaces as a generic ACP stream-disconnect error — not as a configuration error at run start.

## Rule

> When launching a Compozy run in `pr-review` mode (or any mode that depends on a configured model), validate the model name against the IDE's actual model list at run start, BEFORE subprocess spawn. Otherwise an entire batch can be wasted on a typo or stale config.

## Operationalization

- At run start, query the configured IDE's `/v1/models` (or equivalent) endpoint and verify the requested model is present.
- If absent, fail loudly with the available list — not a generic ACP error after subprocess startup.
- Same check applies to provider keys, API base URLs, and any stale credential.
- For internal (non-Compozy) tools: every wrapper that passes through model names should validate before spawn, never after.

## Detection signals

- `ACP error -32603: stream disconnected before completion`
- `The model '<name>' does not exist`
- Run shows `status: "failed"` with no per-job evidence — the failure happened before jobs ran.

## Anti-pattern

- Surfacing model errors as generic ACP transport errors.
- Treating model misconfiguration as a transient retry candidate.
- Hardcoding model names in run scaffolding without a validation layer.

## Source

- `.compozy/runs/reviews-session-driver-override-79f260-round-000-20260424-021709`
- `../analysis/analysis_local_runs.md` lesson LL-5
