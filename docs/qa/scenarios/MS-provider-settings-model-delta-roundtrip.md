---
id: MS-provider-settings-model-delta-roundtrip
area: MS
title: Provider Settings preserves model deltas and validation semantics
persona: Ada
journey: J-20
expected: A Provider Settings PUT with unchanged curated membership persists only explicit five-rate and reasoning deltas, survives restart, leaves unchanged catalog enrichment out of config, and rejects negative or non-finite rates as caller validation without mutation.
entry_points: Provider Settings HTTP+UDS; `compozy config show`; model catalog all-view readback
qa_status: blocked-verify
bug_ids: BUG-20260729-provider-model-pricing-roundtrip;BUG-20260729-provider-model-validation-status
fix_status: pending
retest_status:
fix_commits:
evidence: unpublished-local-artifact/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-056
---

Added by the MS-056 QA repair impact flag. The rebuilt candidate passed the originating replay;
execute this tracker row in the next QA cycle after the governed fix commit exists.

## Claude auto-compaction window

Set `providers.claude.auto_compact_window = 400000`, read it through Provider Settings,
and reload config. An agent with `auto_compact_window: 200000` uses its own window;
an agent without the key inherits 400000. Capture session/new, session/load and
session/resume metadata and verify `CLAUDE_CODE_AUTO_COMPACT_WINDOW` has that value.
Reject 0, 99999, 1000001 and malformed strings without changing saved configuration.
With both keys absent, no variable is injected. Retired session.compaction settings
continue to archive; this new provider key remains active.

### Lean Claude environment

Create a Claude worker definition with `lean_environment: true`, start it, release
its runtime, and resume it. Verify the native startup options keep repository
instructions and configured MCP servers while excluding ambient MCP servers,
claude.ai connectors and auto-memory. Repeat with the key absent: user defaults
stay unchanged. Observe system and heartbeat triage starts using the lean options
automatically. Repeat with a non-Claude provider and verify no Claude options leak.
