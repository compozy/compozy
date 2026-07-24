---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: internal/cli/task_group_picker.go
line: 251
severity: medium
author: claude-code
provider_ref:
---

# Issue 011: One bad workflow dir hard-fails the whole review-fix picker

## Review Comment

`buildReviewFixTargetPickerOptions` iterates every directory from
`listTaskSubdirs` and aborts the entire picker on the first per-slug error:

```go
target, err := resolver.Resolve(ctx, workspaceRoot, slug)
if err != nil {
    return nil, fmt.Errorf("resolve review target %s: %w", slug, err)
}
```

Three distinct failure sources reach that pattern: `resolver.Resolve` (line 251),
`EvaluateReadiness` inside `buildTaskGroupPickerOption` (line 306), and
`reviewRoundPickerSummaryAcrossRounds` → `ReadReviewEntries` →
`ParseReviewContext` (lines 328 and 362).

Failure: (a) any initiative with a malformed plan returns `ErrInvalidPlan`;
(b) any single legacy or hand-written `issue_NNN.md` missing `status:` front
matter returns `ErrLegacyReviewMetadata` / "review front matter missing status"
(`internal/core/reviews/parser.go:57-82`). Either one makes flagless
`compozy reviews fix` fail with "interactive form failed: load review target
picker: ..." for *every* workflow, including healthy unrelated ones, until the
offending file is repaired by hand.

The sibling task-run wizard already degrades instead of failing:
`readTaskRunWizardPlan` (`internal/cli/tasks_run_wizard.go:570`) returns
`(Plan{}, false)` on any error and falls back to an ordinary row, and
`ClassifyTarget` (`internal/core/taskgroups/resolver.go:28`) has a documented
`ErrInvalidPlan` degradation path. The review picker is the outlier.

Fix: skip the offending slug or render it as a blocked row carrying the parse
error as its reason, matching the wizard's degradation. Add a test with one
malformed issue file asserting the other targets still list.

## Triage

- Decision: `VALID`
- Root cause: `buildReviewFixTargetPickerOptions` processed every slug inline in one
  loop and returned on the first per-slug error, so a single failure aborted the
  entire picker. Three failure sources reach that loop: `resolver.Resolve`
  (`ErrInvalidPlan` for a malformed plan), `EvaluateReadiness` inside
  `buildTaskGroupPickerOption`, and `reviewRoundPickerSummaryAcrossRounds` →
  `ReadReviewEntries` → `ParseReviewContext` (`ErrLegacyReviewMetadata` / "review
  front matter missing status") for a malformed `issue_NNN.md`. Any one made
  flagless `compozy reviews fix` fail with "load review target picker: ..." for
  *every* workflow, including healthy ones, until the offending file was repaired
  by hand. The task-run wizard already degrades (`readTaskRunWizardPlan` returns
  `(Plan{}, false)` and falls back to a plain row); the review picker was the
  outlier.
- Fix approach: extract per-slug row building into
  `buildReviewFixTargetPickerRowsForSlug`; when it returns an error, append a
  single blocked row (`reviewFixTargetErrorRow`) carrying the parse/plan error as
  its `SelectionBlockedReason` and marked `[⊘]`, then continue to the next slug.
  Healthy targets still list and stay selectable; the broken one is visible but
  unselectable with the reason shown, matching the wizard's degradation.
- Tests: added `TestReviewFixPickerDegradesOnMalformedIssueFile` — one workflow
  with a status-less `issue_001.md` plus a healthy workflow; asserts the picker
  builds without error, the healthy target is selectable, and the broken target is
  a blocked row whose reason surfaces the parse failure.
