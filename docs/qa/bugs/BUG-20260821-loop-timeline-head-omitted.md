# BUG-20260821-loop-timeline-head-omitted: Resume error hides the real timeline head

- **Status:** verified
- **Impact (user-side):** Blocks-Recovery
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Ada, headless Loop operator
- **Journey Step:** J-operate-loop-run-headless, resume after a stored sequence
- **Scenarios:** LP-run-read-agent-journey
- **Found:** 2026-08-21 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md

## Summary

`compozy loop events --after 999` returned only `timeline_position_beyond_head`, so an operator could
not discover the real `head_seq` needed to recover. The domain error already carried both numbers;
the HTTP/UDS error mapper discarded them.

## Evidence

- Before: `qa/headless/after-beyond-head-human.txt`
- After: `qa/headless/after-beyond-head-fixed-human.txt`,
  `qa/headless/after-beyond-head-fixed.json`, and `qa/headless/after-beyond-head-fixed-http.txt`

## Fix

- **Root cause:** The public error mapping collapsed `TimelinePositionError` to its sentinel code.
- **Fix commit:** `37c101d`
- **Regression test:** `TestLoopReadHandlersMapping/Should_map_a_plain_sequence_resume_error`

## Verification

- **Retested:** 2026-08-21 in the isolated runtime lab
- **Result:** CLI human output names position `999` and head `10`; JSON, HTTP, and UDS preserve the
  stable code plus `details.position` and `details.head_seq`.
