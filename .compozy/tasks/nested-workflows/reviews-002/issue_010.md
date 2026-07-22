---
provider: manual
pr:
round: 2
round_created_at: 2026-07-22T15:39:03Z
status: resolved
file: skills/cy-create-tasks/references/task-template.md
line: 37
severity: medium
author: claude-code
provider_ref:
---

# Issue 010: Relevant paths are not classified or validated

## Review Comment

`Implementation Details`, `Relevant Files`, and `Dependent Files` render undifferentiated path lists. They do not say whether a path exists, is proposed, is generated, or is only a possible architectural location. This makes filename guesses look like approved topology and can send implementers to renamed or nonexistent files.

Store and render a classification for every path: existing file to inspect/modify, proposed new file, generated artifact, or possible architectural location. Validate every path marked existing at generation time and report missing or renamed paths as blocking errors. Keep proposed locations advisory unless an approved TechSpec mandates them, avoid making completion depend on an arbitrary suggested filename, and derive dependency paths from repository analysis rather than guesses.

## Triage

- Decision: `VALID`
- Notes: `task-template.md` currently renders `Relevant Files` and `Dependent Files` as bare path bullets. It neither records whether a path is existing, proposed, generated, or only a possible location nor requires existing paths to be checked against the repository. This can turn an exploration guess into an apparent implementation contract. Update the template to require an explicit classification on every path, block generation when an `existing` path cannot be resolved, keep proposed/possible locations advisory unless the approved TechSpec mandates them, and require dependent paths to come from repository analysis. Add a bundle regression test for these rules. The generated review worktree makes the Playwright daemon socket path 244 bytes, beyond macOS's 104-byte `sun_path` limit, so the unchanged full gate must run from an exact short-path clone. Its Go worktree tests also need the standard short `TMPDIR=/tmp` root to avoid `Result too large` from long test temp paths; both verifier constraints are unrelated to the scoped fix.
