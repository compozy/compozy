# Issue #835 — changed-files stability

- Scope: changed-files visibility during parent and subagent streaming, settlement, and reload.
- Status: scoped QA passed; PR CI/reviews pending.
- Persona: Théo, supervising edits and parallel reviews on a desktop, en-US, local network.
- Driver: focused Playwright probe against an isolated daemon with deterministic ACP mock agents (explicitly requested); no browser/API mocks and no operator daemon or data.
- Charter: send a file-edit request with four parallel reviews, watch successive progress updates, inspect the settled file summary, and reload. Adjacent canary: live subagent cards remain visible while completed parent work folds.
- Lab manifest: `/private/tmp/compozy-issue-835/compozy-issue-835-flicker-20261019-190031-233163-lab/qa-artifacts/qa/bootstrap-manifest.json`.

| Journey | Scenario | Status |
| --- | --- | --- |
| Observe live edits and four reviewers, then inspect settled files | ET-web-session-transcript-calm-grammar | Scoped pass |
| Reload the settled transcript and inspect cards | ET-web-native-subagent | Scoped pass |

## Reproduction before repair

- The existing timeline logic suite reproduced a visible changed-files row with a live subagent and no explicit active turn.
- A real assistant-ui runtime hook test in the existing subagent wiring suite reproduced `working.turnId` switching from `turn-1` to `child-turn` when the trailing progress event changed attribution.
- Commands: `bunx turbo run test --filter=./web --only -- session-timeline.logic.test.ts` and the equivalent command for `session-subagent-wiring.test.tsx`.

## Repair

Parent activity derives from parent text/reasoning/tools rather than child attribution or progress receipts. The first parent content anchors its turn. Live subagent turns withhold the file roll-up across row-group boundaries, while preserving the existing contract that a settled parent's work may fold with its live child card visible.

## Validation and limits

- Rendered run: session `sess-80bd1ebe94b7d7fc`, four native reviewers, eight child updates. A DOM MutationObserver recorded exactly `[0, 1]` summary-count transitions over 9.545 seconds; four public roster records completed, zero browser page errors, and one summary persisted after reload.
- [Live reviewers](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-835/live.png) and [settled file row](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-835/settled.png). Raw observations, fixture, probe logs and reload capture live beside the manifest.
- `bunx turbo run test --filter=./web --only -- session-timeline.logic.test.ts session-subagent-wiring.test.tsx`: 90 passed. Existing session-thread suite: 167 passed (existing React act/getSnapshot warnings).
- `bunx turbo run typecheck --filter=./web --only` and `bunx turbo run lint --filter=./web --only`: passed; lint zero warnings/errors. Turbo reported a shared-cache permission warning after successful tasks.
- `bunx react-doctor web --scope files --base 116b69ea4dfe88731910900c4f9f7eb71513caa6 --verbose --no-supply-chain --no-score`: no issues in four changed source/test files.
- Source daemon build and affected web production build passed. Web build retained existing CSS highlight, chunk-size and terminal import warnings.
- The first browser probe polled the roster for child text; the public transcript owns that text. Corrected the probe and reran successfully with a fresh session, without changing production code.
- Global local gates and full E2E are prohibited for this assignment; CI owns delivery gates. The generic QA audit retains C14 (missing full local gate evidence) for this explicit exception. No claim of a passing global gate is made.
- The deterministic ACP run does not establish external-provider compatibility or a physical file write; the fixture reports an edit through the real protocol. Other scenario branches were not revalidated.

Teardown completed through the manifest-owned helper: `TEARDOWN_ALL_CLEAN=true`; only this lab and its registered processes were stopped.

## Review follow-up: stopped history

Greptile identified that a fresh load of a stopped session has no SSE snapshot. The empty roster left recorded children unconfirmed/live and suppressed saved file summaries. Reproduced against the first build in a new browser: expected one summary, received zero. Reused the complete, paginated roster read for stopped sessions with no polling interval; active prompt polling remains unchanged. The existing roster-control suite now verifies one-time settlement hydration without interval requests. Focused timeline/wiring/roster suites: 97 tests passed; affected web typecheck/lint and React Doctor passed. The real stopped-session re-walk is recorded below.

Re-walk passed on the repaired bundle: a fresh browser loaded stopped session `sess-80bd1ebe94b7d7fc`, displayed one summary and expanded its file row. A fresh streaming session `sess-343c9911695392ad` again produced exactly `[0, 1]` over 9.699 seconds with zero page errors. [Stopped-history evidence](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-835/stopped.png).
