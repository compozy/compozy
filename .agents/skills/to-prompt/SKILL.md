---
name: to-prompt
description: Turn code, issues, or context into a handoff brief for another LLM — full context, zero prescribed solution, written to docs/prompts/. Use when packaging a bug fix, an improvement, or a feature request for an external LLM to implement. Don't use for prompts that already prescribe an implementation, simple one-shot questions, or end-user-facing copy.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# To Prompt

Write a **brief**, not a solution. The brief carries everything the receiving LLM needs to choose the implementation itself — the problem, the current state, the requirements, the constraints — and stops exactly where the "how" begins: the receiving LLM owns the how. Code appears only as evidence of what exists today, never as the fix.

Every brief is a file: `docs/prompts/<YYYYMMDD-HHmm>_<slug>.md` (e.g. `docs/prompts/20260709-1745_fix-auth-redirect.md`).

## Steps

1. **Classify the task** — bug fix, improvement, or feature — and draft a one-paragraph statement of the problem and the outcome wanted.
2. **Gather the evidence.** Read the code paths involved; quote current code as it exists; capture error messages, logs, and stack traces verbatim; note environment and recent changes. *Done when:* every claim in the brief is backed by something you actually read or ran.
3. **Write the brief from the template.** Copy `assets/brief-template.md` to `docs/prompts/<YYYYMMDD-HHmm>_<slug>.md` (create the directory if absent; slug = lowercase-hyphenated summary; timestamp = now). Fill every section following the template's per-type guidance comments, then delete the comments. *Done when:* every section is filled or carries an explicit `Unknown — <reason>` line — a silent gap reads as "nothing to say" to the receiver — and no guidance comment remains.
4. **Sweep for leaked solutions.** Reread the file; wherever a suggested approach, a pattern to follow, a step-by-step plan, or after-state code slipped in, delete it and keep only the requirement it was smuggling. *Done when:* the brief answers "what" everywhere and "how" nowhere.
5. **Hand off.** Reply with the file path and a one-line summary of what the brief asks for.

## Bundled files

- `assets/brief-template.md` — the brief's skeleton and per-type coverage guidance; the single source of truth for what a complete brief contains.
