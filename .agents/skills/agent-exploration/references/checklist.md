# Output Validation Checklist

Run this checklist after every research round, before authoring `summary.md`. Every item must pass; failing items trigger a re-dispatch of the offending slice.

## 1. Runtime

- [ ] Every slice ran as a native subagent of the current harness.
- [ ] No silent substitution: every slice ran on the model the operator pinned, or the inherited session model when none was pinned.

## 2. Inputs

- [ ] `--path` resolved to an absolute path that exists.
- [ ] `--agents` is between 1 and 8 inclusive.
- [ ] `--prompt` is non-empty and quoted in every dispatched slice prompt verbatim.
- [ ] `--model` was forwarded per-slice only when the operator pinned one; otherwise slices inherited the session's model and reasoning effort.
- [ ] `<path>/analysis/` exists before dispatch.

## 3. Scout

- [ ] The parent performed a read-only scout of 8–15 tool calls.
- [ ] Slice count matches `--agents` OR was reduced with operator notice.
- [ ] Slices are non-overlapping and independently answerable.
- [ ] Every slice has a two-digit ordinal and a kebab-case slug.

## 4. Dispatch

- [ ] Every slice was dispatched as one native subagent call, all in a single parallel batch (no staggering).
- [ ] Every slice prompt was written to its own file under `<path>/.dispatch/prompts/` (the round's audit trail and re-dispatch source).
- [ ] Every slice prompt embedded `assets/explorer-prompt.md`, `references/dispatch-rules.md`, and the seven-section schema from `assets/analysis-template.md` verbatim.
- [ ] Every slice prompt named slice scope, slug+ordinal, and target path.
- [ ] Every slice finished clean — the subagent returned its written-path confirmation. Failures triggered slice re-dispatch.

## 5. Files

- [ ] Exactly `N` files exist under `<path>/analysis/` matching the dispatched ordinals/slugs.
- [ ] No file is empty or stub-only.
- [ ] No file was written outside `<path>/analysis/`.

## 6. Schema

- [ ] Every file contains all seven sections (Overview, Mechanisms/Patterns, Relevant Sources, Transferable Patterns, Risks/Mismatches, Open Questions, Evidence).
- [ ] No section is empty without a one-line gap-note and a matching Open Question.
- [ ] At least one cited source per file was sample-checked and confirmed real.

## 7. Summary

- [ ] `summary.md` is parent-authored, not produced by a dispatched agent.
- [ ] `summary.md` cites every slice file by path.
- [ ] Convergences and Divergences sections both have content (or explicit notes that none surfaced).
- [ ] Recommended Next Steps cite the slice file(s) that support them.
