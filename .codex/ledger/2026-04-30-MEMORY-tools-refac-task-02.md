Goal (incl. success criteria):

- Execute AGH tools-refac Task 02: add bundled agh-tools-guide, wire startup tools guidance section, update shipped catalog/setup/docs guidance, add deterministic tests, update tracking/memory, and create one local commit after clean verification.

Constraints/Assumptions:

- Must use cy-workflow-memory before editing code and before finish; cy-execute-task governs end-to-end workflow; cy-final-verify before completion/commit.
- Must read AGH guidance, PRD docs, \_techspec.md, \_tasks.md, ADR-001, and workflow memory before editing code.
- Must run make verify successfully before completion or commit.
- Must not run destructive git commands without explicit permission.
- Keep scope to guidance assets and prompt assembly, not broadening the tool catalog.

Key decisions:

- Session ledger lives in looper .codex/ledger because that is the provided workspace policy scope; AGH workflow memory remains in the PRD memory paths.

State:

- Task 02 implementation is committed and post-commit verification passed on the clean committed tree.

Done:

- Loaded cy-workflow-memory, cy-execute-task, cy-final-verify, golang-pro, testing-anti-patterns skill instructions.
- Read prior tools-refac Task 02 ledger as read-only cross-agent context.
- Read AGH root/internal/site guidance, workflow memory, task file, \_techspec.md, \_tasks.md, ADR-001/ADR-002, and competitor notes.
- Pre-change baseline: no committed agh-tools-guide SKILL.md, no HarnessPromptSectionTools, catalog usage still says `agh skill view`, and agh-network startup guidance says CLI-only.
- Added bundled `agh-tools-guide` and updated setup/catalog/network/docs guidance toward tool-first discovery and invocation.
- Wired `HarnessPromptSectionTools` through runtime signals, section resolution, and startup descriptors.
- Added deterministic unit/integration coverage for catalog wording, bundled skill visibility, prompt assembly, and runtime section ordering.
- Focused Go test commands passed for `internal/skills`, `internal/skills/bundled`, and the targeted daemon prompt/harness tests.
- First `make verify` run failed in `golangci-lint` on Task 02 code: gocritic `appendCombine` in `internal/daemon/prompt_sections.go`; fixed by combining the new tools/network descriptor append.
- Second `make verify` passed: format/oxlint clean, typecheck clean, Vitest `257` files / `1838` tests passed, web build completed with existing chunk-size notice, Go lint `0 issues`, Go tests `DONE 7021 tests`, package boundaries respected.
- Fresh ad-hoc coverage rerun is currently blocked by unrelated concurrent task lease/store edits that appeared after `make verify`.
- Updated Task 02 workflow memory, shared workflow memory, task status/checklists, and `_tasks.md` tracking. These tracking files remain outside the planned commit.
- Created local commit `6640f66a feat: add tools guidance startup section` with only the Task 02 code/docs files.
- Post-commit `make verify` passed in clean worktree `/tmp/agh-task02-verify.x0oEQU`: format/oxlint clean, typecheck clean, Vitest `257` files / `1838` tests passed, web build completed with existing chunk-size notice, Go lint `0 issues`, Go tests `DONE 7019 tests`, package boundaries respected.

Now:

- Final status sanity check and response.

Next:

- None for Task 02.

Open questions (UNCONFIRMED if needed):

- none.

Working set (files/ids/commands):

- /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/task_02.md
- /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/\_techspec.md
- /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/\_tasks.md
- /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/adrs/adr-001-agent-tool-surface.md
- /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/memory/MEMORY.md
- /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/memory/task_02.md
- /Users/pedronauck/Dev/compozy/agh/internal/daemon/prompt_sections.go
- /Users/pedronauck/Dev/compozy/agh/internal/daemon/harness_context.go
- /Users/pedronauck/Dev/compozy/agh/internal/daemon/boot.go
- /Users/pedronauck/Dev/compozy/agh/internal/daemon/composed_assembler_test.go
- /Users/pedronauck/Dev/compozy/agh/internal/daemon/harness_context_test.go
- /Users/pedronauck/Dev/compozy/agh/internal/daemon/harness_context_integration_test.go
- /Users/pedronauck/Dev/compozy/agh/internal/daemon/daemon_test.go
- /Users/pedronauck/Dev/compozy/agh/internal/skills/catalog.go
- /Users/pedronauck/Dev/compozy/agh/internal/skills/catalog_test.go
- /Users/pedronauck/Dev/compozy/agh/internal/skills/bundled/bundled_test.go
- /Users/pedronauck/Dev/compozy/agh/internal/skills/bundled/skills/agh-agent-setup/SKILL.md
- /Users/pedronauck/Dev/compozy/agh/internal/skills/bundled/skills/agh-network/SKILL.md
- /Users/pedronauck/Dev/compozy/agh/internal/skills/bundled/skills/agh-tools-guide/SKILL.md
- /Users/pedronauck/Dev/compozy/agh/packages/site/content/runtime/core/configuration/agent-md.mdx
- /Users/pedronauck/Dev/compozy/agh/packages/site/content/runtime/core/agents/definitions.mdx
- /Users/pedronauck/Dev/compozy/agh/packages/site/content/runtime/core/network/index.mdx
