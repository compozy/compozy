# BUG-20260928-provider-error-notice-live-duplicate: A rate-limited turn shows its provider notice twice until reload

- **Status:** open
- **Impact (user-side):** Cosmetic
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-14: a turn fails as rate_limited while the window is open
- **Scenarios:** RT-provider-error-handoff; ET-web-session-continue
- **Found:** 2026-09-28 · **Report:** docs/qa/reports/2026-09-28-session-continue-fork-exec-b2.md

## Summary

Sending "rate limit this turn" from the composer (`handoff-agent`) renders two identical "acpmock is rate limited — … Continue with another agent…" notices. After a reload only one remains.

## Root cause (hypothesis)

The daemon records both an `error` event (with `provider_error`) and a `transcript_marker.provider_failure` for the turn. `projectTranscriptMessages` (`web/src/systems/session/lib/session-transcript-query.ts`) dedupes them by `turn_id`, but only over the durable transcript pages. While the prompt stream is live, the streamed `error` part and the tailed marker are merged without that projection. This predates the feature (it applies to every provider-error next action). Owning layer: the live/durable transcript merge.

## Evidence

docs/qa/evidence/2026-09-28-session-continue-fork-b2/live-duplicate-provider-notice.png; transcript read shows one `error` (seq 10) + one marker (seq 11), same `turn_id`.
