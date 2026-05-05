Goal (incl. success criteria):

- Complete network-threads task_04 in /Users/pedronauck/Dev/compozy/agh2: add SQLite conversation schema migration, store DTO validation foundation, required migration/DTO tests, tracking updates, clean make verify, and one local commit.

Constraints/Assumptions:

- Follow user/system/developer instructions, repo AGENTS.md/CLAUDE.md/internal guidance, task_04.md, \_techspec.md, ADRs, and workflow memory.
- No destructive git commands (`restore`, `checkout`, `reset`, `clean`, `rm`) without explicit permission.
- Must read workflow memory before code edits and update task memory as decisions/learnings/touched surfaces change.
- Must use required skills: cy-workflow-memory, cy-execute-task, cy-final-verify; task also requires agh-schema-migration, agh-code-guidelines, golang-pro, agh-test-conventions, testing-anti-patterns.
- Automatic commit is enabled only after clean verification, self-review, and tracking updates.

Key decisions:

- UNCONFIRMED until repository/schema/TechSpec discovery is complete.

State:

- Discovery/pre-implementation.

Done:

- Identified implementation target repo as /Users/pedronauck/Dev/compozy/agh2.
- Scanned existing ledgers for network-threads overlap; relevant prior ledgers include network-threads, network-wire-model, and network-work-primitives.

Now:

- Read workflow memory, required skills, repo instructions, PRD/TechSpec/ADRs, and current store/globaldb code before edits.

Next:

- Build a focused plan, implement DTOs/migration/tests, run targeted tests then make verify, update tracking/memory, self-review, commit.

Open questions (UNCONFIRMED if needed):

- Whether all project-specific agh-\* skills are available locally under .agents/skills.
- Whether global schema version 17 is still the next free migration in this checkout.

Working set (files/ids/commands):

- Repo: /Users/pedronauck/Dev/compozy/agh2
- Task files: .compozy/tasks/network-threads/task_04.md, \_tasks.md, \_techspec.md, adrs/
- Workflow memory: .compozy/tasks/network-threads/memory/MEMORY.md, memory/task_04.md
