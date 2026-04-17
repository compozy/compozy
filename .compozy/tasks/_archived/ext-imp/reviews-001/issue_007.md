---
status: resolved
file: internal/core/agents/agents_test.go
line: 214
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093952670,nitpick_hash:7bd5afbea14b
review_hash: 7bd5afbea14b
source_review_id: "4093952670"
source_review_submitted_at: "2026-04-11T16:18:34Z"
---

# Issue 007: Test intent and assertions are slightly misaligned on DefaultModel.
## Review Comment

You configure `DefaultModel` in the overlay, but the fixture also sets `model: ext-model`, so this test won’t catch a regression in overlay-provided model defaulting. Consider removing the frontmatter `model` field and asserting `resolved.Runtime.Model == "ext-model"`.

## Triage

- Decision: `valid`
- Notes:
  - Root cause: the test fixture hard-codes `model: ext-model`, so the test does not actually verify that the overlay default model is applied when the agent file omits it.
  - Fix plan: remove the explicit frontmatter model from the fixture and assert that discovery resolves the runtime model from the overlay default.
  - Resolved: `internal/core/agents/agents.go` now fills an omitted agent model from the selected IDE runtime default, `internal/core/agents/agents_test.go` asserts the overlay-driven default, and the reusable-agent inspect example/test were updated to match the corrected behavior. This required minimal out-of-scope edits in `internal/cli/reusable_agents_doc_examples_test.go` and `docs/reusable-agents.md` to keep documented output accurate. Verified with `make verify`.
