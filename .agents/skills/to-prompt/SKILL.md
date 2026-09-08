---
name: to-prompt
description: Turn code, issues, or context into a handoff brief for another LLM — relevant context and preserved user decisions, written to docs/prompts/. Use when packaging a bug fix, an improvement, or a feature request for an external LLM to implement. Don't use for simple one-shot questions or end-user-facing copy.
disable-model-invocation: true
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# To Prompt

Write a **brief**, not a solution. The brief carries everything the receiving LLM needs to choose the implementation itself — the problem, the current state, the requirements, the constraints — and preserves any implementation constraints or approved plan already supplied by the user. Leave undecided design choices to the receiver; do not erase accepted decisions to manufacture an open-ended problem.

Every brief is a file: `docs/prompts/<YYYYMMDD-HHmm>_<slug>.md` (e.g. `docs/prompts/20260709-1745_fix-auth-redirect.md`).

## Steps

1. **Classify the task** — bug fix, improvement, or feature — and draft a one-paragraph statement of the problem and the outcome wanted.
2. **Gather the evidence.** Read the code paths involved; quote current code as it exists; capture the relevant error/log excerpts, excluding credentials and unrelated private data; note environment and recent changes. *Done when:* every claim in the brief is backed by something you actually read or ran.
3. **Write the brief from the template.** Copy `assets/brief-template.md` to `docs/prompts/<YYYYMMDD-HHmm>_<slug>.md` (create the directory if absent; slug = lowercase-hyphenated summary; timestamp = now). Fill the applicable sections and remove guidance comments. Record consequential unknowns; omit irrelevant sections.
4. **Check decision fidelity.** Separate user-approved constraints from suggestions and unresolved choices. Preserve the requested outcome and authorization; avoid introducing new implementation mandates.
5. **Hand off.** Reply with the file path and a one-line summary of what the brief asks for.

## Bundled files

- `assets/brief-template.md` — the brief's skeleton and per-type coverage guidance; the single source of truth for what a complete brief contains.
