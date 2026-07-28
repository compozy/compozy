# Output Validation Checklist

Run this checklist after every research round, before authoring `summary.md`. Every item must pass; failing items trigger a re-dispatch of the offending slice.

## 1. Installation
- [ ] `~/.compozy/agents/explorer/AGENT.md` exists and matches `assets/AGENT.md` (or a workspace override at `<repo>/.compozy/agents/explorer/AGENT.md` is present and takes precedence).
- [ ] Re-install via `scripts/install-explorer.sh` if the global definition has drifted from the bundled asset.
- [ ] `compozy` binary is reachable on `PATH`.

## 2. Inputs
- [ ] `--path` resolved to an absolute path that exists.
- [ ] `--agents` is between 1 and 8 inclusive.
- [ ] `--prompt` is non-empty and quoted in every dispatched slice prompt verbatim.
- [ ] `--ide` resolved to a value supported by `compozy exec` (defaults to `claude`).
- [ ] `--model` resolved (defaults to `opus`); `--reasoning` resolved to `low|medium|high|xhigh` (defaults to `xhigh`).
- [ ] `<path>/analysis/` exists before dispatch.

## 3. Scout
- [ ] The parent performed a read-only scout of 8–15 tool calls.
- [ ] Slice count matches `--agents` OR was reduced with operator notice.
- [ ] Slices are non-overlapping and independently answerable.
- [ ] Every slice has a two-digit ordinal and a kebab-case slug.

## 4. Dispatch
- [ ] Every slice was dispatched via `compozy exec --agent explorer` with `--ide`, `--model`, and `--reasoning-effort` flags forwarded from the operator's inputs (either through `scripts/dispatch-slices.sh` or one parallel tool call per slice).
- [ ] Every slice prompt was written to its own file under `<path>/.dispatch/prompts/` and passed via `--prompt-file`.
- [ ] Every slice prompt embedded `references/dispatch-rules.md` verbatim and the seven-section schema from `assets/analysis-template.md`.
- [ ] Every slice prompt named slice scope, slug+ordinal, and target path.
- [ ] All `compozy exec` invocations dispatched in parallel (no staggering).
- [ ] Every `compozy exec` exited 0; non-zero exits triggered slice re-dispatch. When `dispatch-slices.sh` was used, its final summary line reads `failed=0/N`.

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
