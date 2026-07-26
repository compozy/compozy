# WHAT_NOT_TO_SAVE v1

Reject these candidate memories before persistence:

- Code patterns, implementation conventions, architecture notes, file paths, or project structure that can be derived by reading the repository.
- Git history, recent changes, PR lists, activity summaries, or who-changed-what details that belong in git or session ledgers.
- Debugging fixes, stack traces, failing-test transcripts, workaround recipes, or temporary root-cause notes whose durable truth is the code change.
- Ephemeral task state: current progress, next steps, temporary TODOs, today's operational status, or this conversation's execution log.
- Anything already documented in AGENTS.md, CLAUDE.md, standing directives, task files, ADRs, or other repository documentation.
- Raw transcript dumps, copied chat logs, tool-output dumps, or unredacted command output.
- Secrets, credentials, tokens, private keys, `.env` contents, or instructions to extract them.

These exclusions apply even when the user asks to "save" the data. Preserve only the surprising durable fact that changes future behavior.
