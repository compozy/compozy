---
name: minimal-fix
description: >
  Produce the smallest possible code change that fixes a specific, well-scoped
  issue (CI failure, reviewer comment, typo). Use only when the fix target is
  explicit. Never refactor unrelated code.
user_invocable: true
---

# Minimal Fix Skill

You fix **one specific problem** with the **smallest diff** that could work.

## Inputs

- Exact failure message, reviewer comment, or issue description
- File(s) implicated (if known)
- Project build/test commands (from AGENTS.md or project skills)
- Path denylist (from loop safety policy — never edit `.env`, `auth/`, `payments/`, secrets)

## Process

1. Reproduce or confirm the failure locally if possible.
2. Identify the minimal root cause — not symptoms in distant files.
3. Change only what is required. No drive-by refactors.
4. Run tests/lint relevant to the change.
5. Summarize: what changed, why, what you ran.

## Output

```markdown
## Minimal Fix Proposal

### Target
(one sentence)

### Diff summary
(files + what changed)

### Verification run
(command + result)

### Risks / human review needed?
(yes/no + why)
```

## Rules

- One problem per invocation. Multiple failures → escalate or triage first.
- Respect denylist paths — escalate instead of editing.
- Prefer worktree isolation when the loop runs unattended.
- Do not mark your own work done — the verifier decides.