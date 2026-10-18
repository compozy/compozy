# BUG-20260928-provider-error-notice-live-duplicate: A rate-limited turn shows its provider notice twice until reload

- **Status:** fixed (retested 2026-09-29: one notice while the stream is live)
- **Impact (user-side):** Cosmetic
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-14: a turn fails as rate_limited while the window is open
- **Scenarios:** RT-provider-error-handoff; ET-web-session-continue
- **Found:** 2026-09-28 · **Report:** docs/qa/reports/2026-09-28-session-continue-fork-exec-b2.md

## Summary

Sending "rate limit this turn" from the composer (`handoff-agent`) renders two identical "acpmock is rate limited — … Continue with another agent…" notices. After a reload only one remains.

## Root cause

The daemon records both an `error` event (with `provider_error`) and a `transcript_marker.provider_failure` for the turn. `projectTranscriptMessages` (`web/src/systems/session/lib/session-transcript-query.ts`) dedupes them by `turn_id`, but only over the durable transcript pages. The prompt stream emits the same diagnostic `error` as a `data-compozy-event` part of the stream's own assistant message (`internal/api/core/prompt_stream_emit.go` `emitError`). That message's id never matches a durable id, so `mergeSessionThreadReadModel` appended it after the durable-only dedup and the notice rendered twice. This predates the feature (it applies to every provider-error next action). Owning layer: the live/durable transcript merge.

## Fix

`mergeSessionThreadReadModel` (`web/src/systems/session/lib/session-thread-read-model.ts`) collects the turn ids for which the durable transcript already records a provider failure (diagnostic `error` or `provider_failure` marker) and does not append a runtime tail message whose every part is a provider failure for one of those turns. A failure for a turn the transcript lacks still appends. The marker predicate moved to `lib/provider-error.ts` (`isProviderFailureMarker`, `isProviderFailureRecord`) so both projections share it. Regression: `session-chat-runtime-provider.test.tsx` "Should not append a live provider failure the durable transcript already records" (failed before the fix: the merge returned `stream_assistant_001`). Web E2E-004 still triggers the failure before opening the page, so it does not cover the live merge; the unit case owns it and the web walk owes a retest.

## Evidence

docs/qa/evidence/2026-09-28-session-continue-fork-b2/live-duplicate-provider-notice.png; transcript read shows one `error` (seq 10) + one marker (seq 11), same `turn_id`.

## Retest 2026-09-29

Review round 1 QA re-walk (`docs/qa/reports/2026-09-29-session-continue-fork-r1-rewalk.md`): with the window open, "rate limit this turn" was sent from the `handoff-agent` composer. The live window shows exactly one notice across 10 samples over 4 s (`data-provider-next-action="handoff"`), and one after reload. VC-22 was recaptured on the live state. Evidence: `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-handoff-live.png`.
