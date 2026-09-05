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

> Validate an explicitly selected model before launching an expensive external batch when the provider exposes authoritative availability information. Inherit the current configured model when no override is requested.

## Operationalization

- Reuse a current provider catalog or a cheap supported availability check for explicit model selection; do not invent a `/v1/models` endpoint or require an extra lookup for harness-native inheritance.
- An unavailable model or rejected credential is a configuration error, not an unchanged retry candidate. Report the specific failure without exposing secrets.
- The model names in this incident are historical evidence, not current defaults.

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
