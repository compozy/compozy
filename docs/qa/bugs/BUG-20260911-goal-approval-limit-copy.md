# BUG-20260911-goal-approval-limit-copy: Approval text promises unchanged limits during turn extension

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-26 approve a turn-limit extension
- **Scenarios:** GL-007
- **Found:** 2026-09-11
- **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction and evidence

Catalog Goal looprun-ed59d9b171ca4681 reached needs-approval after one rejected turn, with approval-counter.txt containing 1. The Needs you card said approval continued with the same limits. Web approval actually extended the effective limit from 1 to 2; a concurrent CLI approval was rejected and exactly one successor turn completed. Before screenshot: goal-extension-parked.png. Runtime evidence: goal-extension-second-turn-events.json and goal-extension-approval-sse.txt.

## Cause and fix

The shared card fallback asserted unchanged limits for every approval kind. Replace that unsupported promise with the true generic result: approval lets the run continue, rejection ends it. Server-supplied request prompts and all controls remain authoritative. The persona walk ended and the worker stopped before editing. This is a bounded one-string copy repair, with no behavior, API, config, permission, migration, native-tool or official-skill change; cross-surface impact is not applicable — editorial only.

## Validation

Owning surface: Loop Run Needs you card. Existing suite: web/src/systems/loops/components/__tests__/loop-run-page.test.tsx. No prose-only assertion is added; before/after real replay owns copy proof. Rebuilt Web passed. Fresh catalog Run looprun-c02be787f8926b20 reached needs-approval with its independent counter1; corrected fallback is visible after reload in goal-approval-copy-fixed-ready.png. Reject & halt produced blocked with no second turn; refreshed Web and independent file match. The idle worker was explicitly stopped. Original approval acceptance remains valid. Final make gate passed all affected lanes, including the existing Loop run-page suite and all7205 Web tests.

Fix commit: cb7867d9a.
