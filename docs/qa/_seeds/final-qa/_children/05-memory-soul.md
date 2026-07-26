---
name: 05-memory-soul
description: AGH pre-release QA — memory + consolidation + soul module. Real-LLM scenarios required. Read-only research deliverable.
type: qa-child
module: memory-soul
owner: pre-release-qa
references:
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/openclaw-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/hermes-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/agent-soul/_techspec.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/agent-soul/_techspec_soul.md
---

# 05 — Memory + Consolidation + Soul QA

## 1. Module scope

This child stresses every documented invariant for AGH's persistent memory
runtime, the dream-style consolidation cascade, and the agent-soul (per-agent
authored persona + per-agent memory directory) feature shipped on
`2026-05-01`. Every scenario hits real Claude Code subagents, real SQLite
catalogs, and real consolidation locks — never mocks for the assertions that
matter.

Packages in scope (file:line citations are repo-absolute):

| Surface                          | Path                                                                          | Authoritative API                                                                                                                                                                                                                                                                                                                                       |
| -------------------------------- | ----------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Memory store (dual scope)        | `/Users/pedronauck/Dev/compozy/agh/internal/memory/store.go`                  | `Store.Write` (`internal/memory/store.go:149`), `Read` (`:117`), `Delete` (`:175`), `Search` (`:327`), `Reindex` (`:377`), `LoadIndex` (`:281`), `History` (`:480`), `HealthStats` (`:417`), `ForWorkspace` (`:81`), `EnsureDirs` (`:98`)                                                                                                                |
| Memory taxonomy + scope          | `/Users/pedronauck/Dev/compozy/agh/internal/memory/types.go`                  | `Type` constants (`internal/memory/types.go:14-23`), `Scope` constants (`:28-33`), `Header` (`:50-58`), `DefaultScopeForType` (`:237`), `Header.Validate` (`:281`)                                                                                                                                                                                       |
| Recall augmenter                 | `/Users/pedronauck/Dev/compozy/agh/internal/memory/recall.go`                 | `NewRecallAugmenter` (`internal/memory/recall.go:22`), `buildRecallBlock` (`:61`), constants `maxRecallResults=3`, `maxRecallCharacters=1500`, `RecallAugmenterBudget=1500` (`:13-17`)                                                                                                                                                                   |
| Catalog (FTS5 + ranking)         | `/Users/pedronauck/Dev/compozy/agh/internal/memory/catalog.go`                | `catalog.search` (`internal/memory/catalog.go:458`), schema (`:34-87`), bm25 ordering (`:497`), scope filter (`:733`), `logEvent` (`:523`), `listOperations` (`:568`)                                                                                                                                                                                    |
| Prompt assembly + index          | `/Users/pedronauck/Dev/compozy/agh/internal/memory/assembler.go`              | `Assembler.PromptSection` (`internal/memory/assembler.go:50`), `Assemble` (`:98`), constants `defaultIndexLines=200`, `defaultIndexBytes=25_000` (`internal/memory/store.go:23-25`)                                                                                                                                                                      |
| Staleness                        | `/Users/pedronauck/Dev/compozy/agh/internal/memory/staleness.go`              | `FreshnessWarning` (`internal/memory/staleness.go:38`), `freshnessWarning` (`:28`), 1-day threshold (`:30`)                                                                                                                                                                                                                                              |
| Consolidation gates              | `/Users/pedronauck/Dev/compozy/agh/internal/memory/dream.go`                  | `Service.ShouldRun` (`internal/memory/dream.go:172`), `Service.Run` (`:217`), `timeGatePasses` (`:306`), `scanCompletedSessionsSince` (`:314`), defaults `defaultMinHours=24` / `defaultMinSessions=3` (`:19-21`), `ErrLockUnavailable` (`:25-27`)                                                                                                       |
| Consolidation lock               | `/Users/pedronauck/Dev/compozy/agh/internal/memory/lock.go`                   | `ConsolidationLock` (`internal/memory/lock.go:23-29`), `TryAcquire` (`:63`), `Release` (`:123`), `Rollback` (`:132`), `LastConsolidatedAt` (`:45`), `defaultLockStaleAge=time.Hour` (`:18`), file name `.consolidate-lock` (`:16`)                                                                                                                       |
| Consolidation runtime            | `/Users/pedronauck/Dev/compozy/agh/internal/memory/consolidation/runtime.go`  | `Runtime.Trigger` (`internal/memory/consolidation/runtime.go:97`), `Start` (`:119`), `EnqueueCheck` (`:161`), `Shutdown` (`:186`), `runCheck` (`:203`), `NewSessionSpawner` (`:250`)                                                                                                                                                                     |
| Soul resolver / parser           | `/Users/pedronauck/Dev/compozy/agh/internal/soul/soul.go`                     | `Resolve` (`internal/soul/soul.go:159`), `Parse` (`:201`), allowed fields (`:697`), forbidden owners (`:704-726`), digest (`:483`), `ResolvedSoul` (`:53-64`), `Diagnostic` (`:127-134`), `safeSourcePath` (`:590`), `digestPrefix="agh.soul.v1\n"` (`:27`)                                                                                              |
| Soul authoring / persistence     | `/Users/pedronauck/Dev/compozy/agh/internal/soul/persistence.go`              | `Snapshot` (`internal/soul/persistence.go:43-54`), `Revision` (`:87-102`), `RevisionAction` (`:32-41`), `NewConfigProvenance` (`:120`), `SnapshotFromResolved` (`:145`)                                                                                                                                                                                  |
| Situation surface                | `/Users/pedronauck/Dev/compozy/agh/internal/situation/service.go`             | Soul snapshot store interface (`internal/situation/service.go:64-67`), compact projection rendering                                                                                                                                                                                                                                                     |
| Lifecycle hooks                  | `/Users/pedronauck/Dev/compozy/agh/internal/hooks/events.go`                  | `HookSessionPreCreate` / `HookSessionPostCreate` / `HookSessionPreStop` / `HookSessionPostStop` (`internal/hooks/events.go:54-59`); soul-related hooks `HookAgentSoulSnapshotResolved` (`:85`), `HookAgentSoulMutationAfter` (`:86`); subprocess executor 5s default (`internal/hooks/executor_subprocess.go:23,224`), pipeline (`:37-172`)               |
| Memory config                    | `/Users/pedronauck/Dev/compozy/agh/internal/config/config.go`                 | `MemoryConfig` (`internal/config/config.go:145-150`), `DreamConfig` (`:152-159`), defaults (`:461-468`), `MemoryConfig.Validate` (`:1064`)                                                                                                                                                                                                               |
| CLI                              | `/Users/pedronauck/Dev/compozy/agh/internal/cli/memory.go` + `authored_context.go` | `agh memory list` (`:200`), `read` (`:229`), `write` (`:328`), `delete` (`:399`), `search` (`:271`), `reindex` (`:439`), `consolidate` (`:482`), `health` (`:111`), `history` (`:147`); `agh agent soul inspect/validate/write/delete/history/rollback` (`internal/cli/authored_context.go:22-264`)                                              |

Out of scope (covered by other children):

- Full session lifecycle / Manager state machine (module 03).
- Coordinator bootstrap and `task_runs.metadata_json` provenance writes
  (covered as adjacent invariants in module 04 — autonomy kernel).
- Heartbeat (`HEARTBEAT.md`) wake policy, session health, wake events —
  covered by a sibling child (`05b-heartbeat`) when authored. This child
  intentionally does NOT exercise heartbeat invariants.
- AGH Network channels / peers (module 06).

## 2. Authoritative invariants under test

Every invariant maps back to `internal/CLAUDE.md` (lines 123-128) or to the
in-repo source. Coverage IDs follow openclaw lowercase dotted/dashed style.

| Coverage ID                        | Invariant                                                                                                                                                                            | Source                                                                                                                                  |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------- |
| `memory.taxonomy`                  | Memory types are exactly `user | feedback | project | reference`. Other types are rejected at frontmatter parse time.                                                                | `internal/memory/types.go:14-23`, `Validate` (`:225-233`)                                                                                |
| `memory.scope.dual`                | Scopes are exactly `global` and `workspace`. `agent` scope is realized as a per-agent directory under the workspace soul (Soul authoring), NOT as a memory `Scope` enum value.       | `internal/memory/types.go:28-33`; `internal/CLAUDE.md:126`                                                                              |
| `memory.default-scope`             | `user`/`feedback` default to global scope; `project`/`reference` default to workspace scope.                                                                                          | `internal/memory/types.go:237-248`                                                                                                       |
| `memory.atomic-write`              | `Store.Write` validates frontmatter then writes atomically; partial files never appear on disk; `MEMORY.md` index is rebuilt deterministically afterward.                            | `internal/memory/store.go:149-172`, `fileutil.AtomicWriteFile`                                                                          |
| `memory.index-cap`                 | `MEMORY.md` index is bounded to `defaultIndexLines=200` lines and `defaultIndexBytes=25_000` bytes when injected into the prompt; truncation logged.                                  | `internal/memory/store.go:22-25,300-322`                                                                                                |
| `memory.recall-bounded`            | Recall augmenter prepends at most 3 entries totaling ≤1500 chars before the user message.                                                                                            | `internal/memory/recall.go:13-17,82-89`                                                                                                  |
| `memory.recall-precedence`         | Recall ranking uses bm25 scoring + tie-break on `updated_at DESC, filename ASC`. Workspace + global both visible when caller is in a workspace; collisions ranked, never deduped silently. | `internal/memory/catalog.go:497`, fallback scorer `:1061-1075`                                                                          |
| `memory.staleness`                 | Recall results older than 1 day surface a freshness warning to the agent prompt.                                                                                                    | `internal/memory/staleness.go:28-35`                                                                                                     |
| `memory.provenance-survives-consolidation` | Catalog operation log retains an immutable `memory.write` row keyed by `id, type, scope, workspace_root, filename, agent_name, timestamp`. Consolidation does not delete this row. | `internal/memory/catalog.go:78-86,549-565`                                                                                              |
| `memory.workspace-isolation`       | Workspace A memory directory is invisible to a session running in workspace B; recall + list scoped to workspace path.                                                              | `internal/memory/store.go:81-85,506-530`                                                                                                |
| `memory.cli-and-agent-parity`      | Every memory mutation observable via CLI is also reachable from a real agent's tool call (Host API `memory/*`); they share `Backend` (`internal/memory/types.go:125-134`).            | `internal/extension/host_api.go:1879-1914`, `internal/cli/memory.go:200-485`                                                            |
| `consolidation.gate-time`          | Time gate: if `now - LastConsolidatedAt < min_hours`, consolidation is blocked; passes at `min_hours+epsilon`.                                                                       | `internal/memory/dream.go:171-213,306-312`                                                                                              |
| `consolidation.gate-sessions`      | Session gate: requires ≥`min_sessions` completed sessions since the last lock mtime.                                                                                                | `internal/memory/dream.go:189-204,314-361`                                                                                               |
| `consolidation.gate-lock`          | File-lock cascade: `TryAcquire` writes a PID file via tempfile + `os.Link` (atomic); concurrent acquire fails with `ok=false`; cross-process attempt rejects gracefully.             | `internal/memory/lock.go:63-120,242-273`                                                                                                |
| `consolidation.gate-order`         | Gates execute Time → Sessions → Lock in that order; ordering is by computational cost; never replaced by a single heuristic.                                                        | `internal/memory/dream.go:171-213`; `internal/CLAUDE.md:127`                                                                            |
| `consolidation.fires-once`         | Daemon clock advances past gate; consolidation fires at most once per gate-pass; subsequent ticks until the next gate window are no-ops.                                            | `internal/memory/consolidation/runtime.go:148-156,219-246`                                                                              |
| `consolidation.lock-rollback`      | If the spawn errors, lock mtime rolls back to the prior value; ShouldRun gate stays in the same state as before the failed attempt.                                                 | `internal/memory/dream.go:243-258,411-432`; `internal/memory/lock.go:132-144`                                                            |
| `consolidation.diff-auditable`     | Before/after `MEMORY.md` snapshot during a consolidation session is preserved (operation log + Markdown source); diff is reproducible.                                              | `internal/memory/store.go:561-579`, catalog operation log                                                                               |
| `soul.allowed-fields`              | SOUL.md frontmatter accepts only `version, role, tone, principles, constraints, collaboration, memory_policy, tags`. All other fields are diagnostics with code `unsupported_field` or `forbidden_field`. | `internal/soul/soul.go:695-702,704-726`                                                                                                  |
| `soul.forbidden-overlap`           | SOUL.md cannot redeclare AGENT.md / capabilities / task / runtime / network / spawn / config / memory keys; reserved Markdown sections rejected.                                    | `internal/soul/soul.go:704-726,461-481`                                                                                                  |
| `soul.path-isolation`              | SOUL.md path may not escape its workspace root; symlink targets verified via `filepath.EvalSymlinks`.                                                                              | `internal/soul/soul.go:590-650`                                                                                                          |
| `soul.digest-stability`            | Digest is `sha256` over `agh.soul.v1\n` + canonical-JSON frontmatter + `\n` + body; identical content yields identical digest across runs.                                          | `internal/soul/soul.go:483-490`                                                                                                          |
| `soul.compact-projection`          | Compact projection serialized for `/agent/context` is bounded by `agents.soul.context_projection_bytes` (default 2048); truncation deterministic (drop principles → tone → role).    | `internal/soul/soul.go:492-519`; `_techspec_soul.md:557`                                                                                 |
| `soul.managed-only-mutation`       | Direct file writes / hook patches / extension shortcuts cannot mutate `SOUL.md`. Only `SoulAuthoringService` writes both file and revision row in one transaction.                  | `_techspec_soul.md:425,612-621`                                                                                                          |
| `lifecycle.session-pre-create`     | `session.pre_create` hook fires once per session creation in hierarchy + alphabetical order.                                                                                        | `internal/hooks/events.go:54`, `internal/hooks/pipeline.go:59`                                                                          |
| `lifecycle.session-post-stop`      | `session.post_stop` hook fires once on stop; failures fail-open by default; the session row is still finalized.                                                                     | `internal/hooks/events.go:59`, `internal/hooks/pipeline.go:68-79`                                                                       |
| `lifecycle.fail-open`              | Non-required hook subprocess timeout (5s default) does not block the session; error logged.                                                                                         | `internal/hooks/executor_subprocess.go:23,224`; `internal/hooks/pipeline.go:71-77`                                                       |
| `lifecycle.required-fail-closed`   | A `required` lifecycle hook failure halts the dispatch chain with a wrapped error.                                                                                                 | `internal/hooks/pipeline.go:71-77`                                                                                                       |
| `delete-update.flow`               | An explicit `forget X` operation removes the memory file, strips it from `MEMORY.md`, removes the catalog FTS row via the AFTER DELETE trigger, and records `memory.delete` in the operation log. | `internal/memory/store.go:174-189`, `internal/memory/catalog.go:64-67`                                                                  |
| `taxonomy.lint`                    | A `reference` memory must carry pointer-style content (URL/file path); a `feedback` memory must include "Why" and "How to apply" sections; lint surfaces in `agh memory write` validation. | Spec defines lint via convention; current store validates only frontmatter (`internal/memory/store.go:149-167`) — see gap §3              |
| `high-write-rate`                  | Concurrent `Write` calls do not corrupt `MEMORY.md` or the FTS5 index; the global `agh.db` lock + atomic file rename prevent torn writes.                                          | `internal/memory/store.go:149-172,549-579`, `internal/store/globaldb`                                                                   |
| `agent-scope-isolation`            | An `agent_name`-tagged memory written by agent A is recallable only when the recall query is performed in a session bound to agent A; other agents in the same workspace do not surface it. (See gap §3.) | `internal/memory/types.go:50-58` (`Header.AgentName`), `internal/memory/catalog.go:113-123` (column present, search filter pending) |

## 3. Operating model

QA mode is **real-scenario** (per the standing directive on real-scenario QA),
not pytest-style assertions. Every scenario:

- Runs against an isolated AGH_HOME with unique daemon ports + `tmux-bridge`
  socket (per `agh-worktree-isolation` skill).
- Resolves provider auth from the bootstrap manifest according to each
  provider contract: bound-secret, brokered, and explicitly isolated-home
  lanes use `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`, while `native_cli`
  lanes with `home_policy=operator` preserve the operator `HOME` unless the
  scenario explicitly validates isolated provider-home behavior.
- Uses real Claude Code (`claude-opus-4-8` for the writer/asker,
  `claude-sonnet-5` for fanout where indicated) as the subprocess agent
  driver. Real LLM responses are the load-bearing assertion target — SQL
  probes alone are insufficient.
- Emits four artifacts under `.artifacts/qa/<run-id>/mem-XX/`:
  - `mem-XX-report.md` (Worked / Failed / Blocked / Follow-up)
  - `mem-XX-summary.json` (machine-readable)
  - `mem-XX-events.json` (catalog operation log + EventStore window)
  - `mem-XX-output.log` (combined daemon + CLI stdout/stderr)
- Asserts against `memory_catalog_entries`, `memory_operation_log`,
  `agent_soul_snapshots`, `agent_soul_revisions`, AND the agent's actual
  reply text (real-LLM assertion).

Scenarios are numbered `MEM-01..MEM-NN`; each is a fenced `qa-scenario`
block. Reproduce by running them sequentially or in parallel under unique
worktree isolation.

### Coverage gaps surfaced during research

These are documented (not silently absorbed) so the QA execution agent can
either accept them as known-deferred or escalate to the implementing agent
before the run ships:

1. `taxonomy.lint` (`reference` pointer content / `feedback` Why+How
   sections) is a documented expectation but not implemented in
   `Store.Write` (which validates only frontmatter shape — see
   `internal/memory/store.go:149-167`). Scenario MEM-15 explicitly probes
   this; if the runtime does not enforce, the scenario records `outcome=
   follow-up` with the defect filed.
2. `agent-scope-isolation` (per-agent recall filtering by `agent_name`):
   the catalog stores `agent_name` (`internal/memory/catalog.go:120,545-548`)
   but `catalog.search` (`:458-521`) does not yet filter by agent. Scenario
   MEM-13 probes the visible behavior and records the gap.
3. Soul-resident "agent memory directory" (`memory.scope=agent`): the
   product premise refers to a per-agent memory directory beneath the
   resolved soul; the current `internal/memory` enum has only `global` and
   `workspace`. Scenario MEM-08 documents the actual on-disk layout and
   the gap, and runs the existing soul-write/update/delete flow through
   `agh agent soul write/delete/history/rollback`.

## 4. Provider matrix

| Mode                | When                                                                                              | Driver                                                                                                                             |
| ------------------- | ------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `real-claude-code`  | Default for all scenarios that exercise multi-turn recall, agent-driven write, or soul refresh    | `claude-opus-4-8` for the writer; `claude-sonnet-5` for the cross-workspace asker in MEM-12.                                |
| `real-openclaw`     | Cross-driver sanity (MEM-04 only) so we know consolidation isn't Claude-Code-specific             | OpenClaw bundled-plugin runtime via the AGH ACP client.                                                                            |
| `mock-acp` (gate)   | Determinism gate for the consolidation-tick race in MEM-05 only                                   | `internal/e2elane` mock ACP server used to make scheduler tick races deterministic; the daemon, lock, and SQLite are real code paths. |

`mock-acp` is the deterministic dispatcher described in the openclaw
tri-state policy. We do NOT include an `aimock` lane (per openclaw's own
honest framing, it is additive-only).

## 5. Preconditions (apply to every scenario)

- Fresh QA bootstrap via the `agh-qa-bootstrap` skill. Manifest path saved
  to `bootstrap-manifest.json`; `bootstrap.env` exported into the shell
  before any `agh` command.
- Unique `AGH_HOME` per worktree (per the worktree-isolation directive).
- Bound-secret, brokered, and explicitly isolated-home auth staged into
  `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`; `native_cli` providers with
  `home_policy=operator` intentionally use the operator `HOME` / native login
  state unless the scenario explicitly validates isolated provider-home
  behavior.
- Daemon started in background. HTTP / UDS listeners reachable.
- `make verify` is green on the SUT branch before QA runs (per the
  Critical Rules).

Provider-specific config:

```text
AGH_HOME=$HOME/.qa/mem-05/<scenario>/agh-home
AGH_DAEMON_HTTP=127.0.0.1:<unique-port>
AGH_DAEMON_UDS=$AGH_HOME/sock/uds.sock
PROVIDER_HOME=$AGH_HOME/provider-home
PROVIDER_CODEX_HOME=$AGH_HOME/provider-codex-home
AGH_WEB_API_PROXY_TARGET=http://127.0.0.1:<unique-port>
```

For consolidation scenarios that need a fast cycle:

```text
[memory]
enabled = true

[roles.dream]
enabled        = true
agent          = "claude-code"

[memory.dream]
min_hours      = 0.001       # ≈ 3.6s; only acceptable in QA scenarios
min_sessions   = 1           # MEM-04 raises to 5 to cover the sessions gate
check_interval = "2s"
```

## 6. Cleanup (applies to every scenario)

- `agh daemon stop` (or kill PID from manifest).
- Inspect `memory_catalog_entries` and `memory_operation_log`; archive both
  before tearing down AGH_HOME.
- Inspect `agent_soul_snapshots` and `agent_soul_revisions`; archive
  before tearing down AGH_HOME.
- Capture `<workspace>/.agh/memory/` and `~/.agh/memory/` directory
  listings + `MEMORY.md` snapshot.
- Tear down the worktree only after evidence artifacts are written.

## 7. Mandatory scenarios

### MEM-01 — Real Claude Code 3-session feedback memory commit + next-session recall

```yaml qa-scenario
id: mem-01-feedback-memory-multi-session
title: Real Claude Code 3-session conversation about user preferences; after session 3 a `feedback` memory is committed and recalled in turn-1 of session 4
theme: memory.recall
coverage:
  primary:
    - memory.taxonomy
    - memory.default-scope
    - memory.atomic-write
    - memory.recall-bounded
  secondary:
    - memory.cli-and-agent-parity
    - memory.staleness
risk: high
live: true
provider: real-claude-code
preconditions:
  - Fresh AGH_HOME with no existing global/workspace memory.
  - Workspace `wsp-mem01` initialized with a single dummy file so the
    workspace resolver returns a stable root.
  - Real Claude Code agent `claude-code` configured with default
    `memory.scope=global` writing privilege and Host API memory grant.
docs_refs:
  - /Users/pedronauck/Dev/compozy/agh/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/store.go:149
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/recall.go:22
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/types.go:237
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/memory.go:200
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/memory.go:328
steps:
  - Session 1:
    `agh sessions start --agent claude-code --workspace wsp-mem01
     --prompt "I prefer concise PR summaries, three bullet points max,
     no emoji, present-tense verbs."`
    Wait for completion.
  - Session 2:
    `agh sessions start --agent claude-code --workspace wsp-mem01
     --prompt "Also: when you produce TODOs, group them by package
     instead of by priority."`
    Wait for completion.
  - Session 3 (consolidation prompt):
    `agh sessions start --agent claude-code --workspace wsp-mem01
     --prompt "Summarize my recurring style preferences as a feedback
     memory I can rely on going forward. Save it via the memory tool."`
    Wait for completion.
  - Verify via CLI: `agh memory list --scope global -o json` returns
    at least one row with `type=feedback`. Capture the filename.
  - Verify via SQL:
    `agh debug sql 'SELECT type, scope, name FROM memory_catalog_entries
     WHERE type = "feedback"'` returns one row with `scope=global`.
  - Session 4 (recall test):
    `agh sessions start --agent claude-code --workspace wsp-mem01
     --prompt "Write me a one-line PR summary for: refactor: split the
     deploy pipeline into reusable steps."`
    Capture the agent's reply.
expected:
  - After session 3, exactly one new memory file exists at
    `~/.agh/memory/<feedback-filename>.md` with frontmatter
    `type: feedback`, `name: <non-empty>`, optional `description`.
  - The file's body contains the three style cues (concise, three
    bullets, present-tense, no emoji, group-by-package). Phrasing is
    not asserted; presence of the cues is.
  - `memory_operation_log` has exactly one new `memory.write` row with
    `type=memory.write`, `scope=global` and a non-empty `summary`.
  - In session 4, the recall augmenter prepends a "Relevant durable
    memory for this turn:" block (verifiable via `agh debug prompt
    --session <id>` or daemon prompt-trace log) referencing the new
    file's `name` and `[global]` scope tag.
  - Session 4's reply visibly applies the recalled rules: ≤3 bullets,
    no emoji, present-tense verbs. Reviewer reads the transcript and
    answers `worked` / `failed` per the openclaw operator-flow
    pattern; failure on this judgment is a `follow-up` (the model may
    have ignored recall — bug class to triage).
evidence:
  - Filename + body of the written memory.
  - `memory_catalog_entries` + `memory_operation_log` rows.
  - Daemon prompt trace for session 4 turn 1.
  - Session 4 transcript.
failure_signatures:
  - No `feedback` row in `memory_catalog_entries`: write tool not
    invoked or routed wrong. Critical bug.
  - `scope=workspace` written despite default scope rule:
    `memory.default-scope` violated.
  - Recall block absent in session 4: `RecallAugmenter` not wired or
    bm25 score zero (probe `agh memory search "PR summary style"`).
  - Recall block present but agent's reply violates all three rules:
    real-LLM regression or recall payload truncation; capture the
    full prompt.
cleanup:
  - `agh memory delete <feedback-filename> --scope global` to leave
    AGH_HOME clean for the next scenario.
```

### MEM-02 — Recall ranking on global+workspace collision

```yaml qa-scenario
id: mem-02-recall-precedence-global-vs-workspace
title: User memory at workspace scope and at global scope collide on the same query; recall returns both, ranking honors bm25 and tie-break, audit row records the shadow
theme: memory.recall
coverage:
  primary:
    - memory.recall-precedence
    - memory.scope.dual
  secondary:
    - memory.atomic-write
    - memory.cli-and-agent-parity
risk: high
live: true
provider: real-claude-code
preconditions:
  - Workspace `wsp-mem02` initialized.
  - Two memory files seeded with the same `name` ("Test framework
    preference") but different bodies and different scopes:
    1. Global `~/.agh/memory/test-framework-global.md` —
       frontmatter `type: user`, body "Prefer Vitest with v8 coverage."
    2. Workspace
       `wsp-mem02/.agh/memory/test-framework-workspace.md` —
       frontmatter `type: project`, body "This workspace pins Jest 29
       because the runtime forks workers."
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/catalog.go:458
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/catalog.go:497
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/catalog.go:1061
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/recall.go:61
steps:
  - Verify both seeds are present:
    `agh memory list --scope global` and
    `agh memory list --scope workspace --workspace wsp-mem02`.
  - Drive a recall query via real Claude Code:
    `agh sessions start --agent claude-code --workspace wsp-mem02
     --prompt "What test framework should I use here?"`.
  - Capture the daemon prompt trace for turn 1.
  - Capture the agent's reply.
  - Run a direct ranking probe:
    `agh memory search "test framework" --workspace wsp-mem02 -o json`.
expected:
  - The recall augmenter returns BOTH entries (workspace + global) per
    `RecallAugmenter` budget rules, scoped order driven by bm25 score
    DESC then `updated_at DESC` then `filename ASC`
    (`internal/memory/catalog.go:497`).
  - The agent's reply favors the workspace project guidance ("Jest 29
    because forks") over the global preference ("Vitest"). This is the
    spec-aligned precedence: workspace-scoped `project` memory beats
    global `user` memory when both match a workspace query (per
    `internal/CLAUDE.md:125-126` Five-layer skill/memory/agent
    precedence — Workspace > User > Bundled). The reply must
    explicitly mention Jest, not Vitest.
  - The "shadow audit" surfaces in the operation log: a
    `memory.search` row with `summary=query="test framework" results=2`
    is recorded.
evidence:
  - JSON `agh memory search` output (both rows present, ordered).
  - Daemon prompt trace showing both recall entries injected.
  - Session 1 transcript.
  - `memory_operation_log` rows from the search.
failure_signatures:
  - `agh memory search` returns only one row: scope filter wrong;
    `memory.scope.dual` violated.
  - Reply favors Vitest in this workspace: precedence regression
    (workspace did NOT beat global).
  - No `memory.search` row in operation log: catalog event recording
    broken.
cleanup:
  - Delete both seeded memories.
```

### MEM-03 — Consolidation Time gate fires once on clock crossing

```yaml qa-scenario
id: mem-03-consolidation-time-gate
title: Daemon clock advances 24h+ε; consolidation fires exactly once; lock acquired and released; concurrent racing scheduler ticks do not double-fire
theme: consolidation.gate-time
coverage:
  primary:
    - consolidation.gate-time
    - consolidation.gate-order
    - consolidation.fires-once
  secondary:
    - consolidation.gate-lock
risk: critical
live: true
provider: real-claude-code
preconditions:
  - `[memory.dream]` enabled in `<AGH_HOME>/config.toml`,
    `min_hours=0.001`, `min_sessions=1`, `check_interval=1s`.
  - Lock file `<AGH_HOME>/memory/.consolidate-lock` does not yet
    exist (or has mtime >`min_hours` ago; force by touching with
    `touch -t 197001020000 <path>`).
  - One real Claude Code session has completed and stopped (so the
    sessions gate has at least 1).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/dream.go:171
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/dream.go:217
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/lock.go:63
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/consolidation/runtime.go:148
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/consolidation/runtime.go:203
steps:
  - Start the daemon; verify the dream runtime ticker is active
    (daemon log line "daemon: evaluating dream consolidation gates").
  - Wait for two consecutive ticker cycles (≥ 2s).
  - Capture daemon log + lock file mtime + content.
  - Inspect spawned consolidation sessions: there should be exactly
    one `session_type=memory-consolidation` row in `globaldb`.
  - Wait three more cycles. Verify NO new spawn fires (the time gate
    is now blocked by the just-updated lock mtime).
expected:
  - Exactly one daemon log "daemon: starting dream consolidation"
    line for the scenario window, followed by "daemon: dream
    consolidation completed".
  - `<AGH_HOME>/memory/.consolidate-lock` exists, contains a numeric
    PID (the daemon's), and its mtime ≈ "completion time".
  - Subsequent ticks log "memory: time gate blocked consolidation"
    until the next `min_hours` window opens.
  - Exactly one consolidation session row in `globaldb` for the
    window. No duplicate.
evidence:
  - Lock file content + mtime (before/after).
  - Daemon log fragment showing the gate-pass sequence + subsequent
    blocked logs.
  - `globaldb sessions` row dump for the window.
failure_signatures:
  - Two "starting dream consolidation" lines for one window: gate
    not honored or `runMu` mutex bypassed
    (`internal/memory/dream.go:228`).
  - Lock file mtime not updated after success: `Release` did not
    `Chtimes` (`internal/memory/lock.go:228-237`).
  - No log lines: ticker not started; runtime not enabled.
cleanup:
  - Reset config to default `min_hours=24`, `min_sessions=3`.
  - Stop daemon. Archive lock file + log.
```

### MEM-04 — Consolidation Sessions gate fires after 5 touched sessions

```yaml qa-scenario
id: mem-04-consolidation-sessions-gate
title: 5 sessions completed within the time-gate window; sessions gate passes; consolidation fires; ledger summary shows the count crossed
theme: consolidation.gate-sessions
coverage:
  primary:
    - consolidation.gate-sessions
    - consolidation.gate-order
    - memory.cli-and-agent-parity
  secondary:
    - consolidation.fires-once
risk: high
live: true
provider: real-claude-code
preconditions:
  - `[memory.dream]` config: `min_hours=0` (only sessions gate
    relevant), `min_sessions=5`, `check_interval=2s`.
  - Lock mtime preset to a far-past timestamp so the time gate is
    open from the start.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/dream.go:189
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/dream.go:314
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/dream.go:363
steps:
  - Run 4 sequential real Claude Code sessions, each "Summarize this
    repository structure briefly" prompt; let them complete. Verify
    `agh sessions list` shows 4 stopped sessions.
  - Trigger a manual gate evaluation:
    `agh memory consolidate --workspace wsp-mem04`.
  - Expect: command returns "consolidation gates not satisfied"
    (4 < 5 sessions).
  - Run a 5th session; let it complete.
  - Trigger again: `agh memory consolidate --workspace wsp-mem04`.
expected:
  - Daemon log shows
    `"memory: session gate blocked consolidation"
     completed_sessions=4 min_sessions=5` after the 4-session probe.
  - After 5th session, gate passes:
    `"memory: consolidation gates passed" completed_sessions=5`.
  - One consolidation session spawns for the workspace
    (`session_type=memory-consolidation`).
  - `memory_operation_log` records the consolidation cycle's writes
    (if any) with the consolidation session as `agent_name`.
evidence:
  - Daemon log fragment showing both blocked and passed states.
  - `globaldb` sessions list filtered to the window.
  - `memory_operation_log` rows from the consolidation run.
failure_signatures:
  - Gate passes at 4 sessions: `min_sessions` not honored.
  - Gate never passes at 5: `scanCompletedSessionsSince`
    miscounts (`internal/memory/dream.go:314-361`); inspect
    `meta.json` files for missing `state` or `stopped_at`.
cleanup:
  - Reset config; stop daemon.
```

### MEM-05 — Consolidation Lock gate prevents two runs in one AGH_HOME

```yaml qa-scenario
id: mem-05-consolidation-lock-gate
title: Two parallel consolidation triggers in the same AGH_HOME — exactly one acquires; the other returns ErrLockUnavailable; cross-process attempt rejected gracefully
theme: consolidation.gate-lock
coverage:
  primary:
    - consolidation.gate-lock
    - consolidation.fires-once
  secondary:
    - consolidation.lock-rollback
risk: critical
live: true
provider: mock-acp
preconditions:
  - `[memory.dream]` config: `min_hours=0`, `min_sessions=0` so both
    triggers immediately reach the lock gate.
  - One AGH_HOME, daemon NOT running (we exercise direct
    `agh memory consolidate` calls and a separate process probe).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/lock.go:63
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/lock.go:242
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/dream.go:373
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/dream.go:391
steps:
  - Start daemon process A. From two shells, in parallel, both
    targeting the same AGH_HOME:
    `(agh memory consolidate &) ; (agh memory consolidate &)`.
  - Wait for both calls to return.
  - Inspect the lock file `<AGH_HOME>/memory/.consolidate-lock`
    content + mtime.
  - Now simulate a second process holding a stale lock with a dead
    PID: write `99999999\n` (assumed dead PID) to the lock file
    with `os.Chtimes(now-2*time.Hour)`. Run
    `agh memory consolidate`. The stale-lock reclaim path in
    `canReclaim` (`internal/memory/lock.go:207-215`) must release
    and re-acquire.
  - Now write a fresh PID (the running daemon's PID, captured from
    `daemon.json`) to the lock file with `mtime=now`. Run a third
    `agh memory consolidate` in another process. Expect
    `ok=false` from `TryAcquire` because the live PID owns the lock.
expected:
  - Step 1: exactly one of the two parallel calls returns "ran".
    The other returns `dream consolidation is already running`
    (per `Trigger` mapping `ErrLockUnavailable`,
    `internal/memory/consolidation/runtime.go:108-112`).
  - Step 3 (stale-pid reclaim): the new call succeeds; lock content
    updated to the running daemon's PID.
  - Step 4 (live foreign PID): the new call returns
    `dream consolidation is already running`. Lock file is
    untouched (no PID overwrite).
evidence:
  - Both step-1 stdout returns captured.
  - Lock file content + mtime at three points (T0 / after stale /
    after live-foreign).
  - Daemon log fragment for each.
failure_signatures:
  - Both step-1 calls succeed: critical concurrency bug; `mu`
    mutex broken (`internal/memory/dream.go:60,373-388`).
  - Stale-pid path does not reclaim: `canReclaim` regression.
  - Live-foreign-pid path overwrites the lock: cross-process safety
    broken.
cleanup:
  - Reset lock file to absent; stop daemon.
```

### MEM-06 — Recall under 4-way collision: correct entry wins, provenance preserved

```yaml qa-scenario
id: mem-06-recall-collision-4way
title: Four memories of different types share a key term; bm25 ranking + dedup picks the right one; provenance (filename, scope, type, agent_name, mod_time) preserved through the recall payload
theme: memory.recall
coverage:
  primary:
    - memory.recall-precedence
    - memory.recall-bounded
  secondary:
    - memory.taxonomy
    - memory.provenance-survives-consolidation
risk: high
live: true
provider: real-claude-code
preconditions:
  - Workspace `wsp-mem06` initialized.
  - Four memory files seeded, each containing the literal phrase
    "auth refactor" but different types/scopes:
    1. Global `user` — "I prefer JWT-based auth over session cookies."
    2. Global `feedback` — "Past auth refactors broke SSR; verify."
    3. Workspace `project` — "Workspace auth refactor uses Lucia v3."
    4. Workspace `reference` — "Lucia v3 docs:
       https://lucia-auth.com/main/getting-started"
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/recall.go:22
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/recall.go:61
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/catalog.go:497
steps:
  - `agh sessions start --agent claude-code --workspace wsp-mem06
     --prompt "Tell me what to use for the auth refactor here."`
  - Capture the daemon prompt trace.
  - Capture the agent's reply.
  - Independently probe the catalog ranking:
    `agh memory search "auth refactor" --workspace wsp-mem06 -o json
     --limit 10`.
expected:
  - Recall augmenter emits at most 3 entries per
    `maxRecallResults=3` (`internal/memory/recall.go:13`). The
    workspace `project` entry MUST be in the top-3.
  - Each rendered recall line includes the entry's `name` and
    `[scope]` (per `buildRecallBlock` `internal/memory/recall.go:73`).
  - Provenance for each recalled entry is preserved end-to-end:
    `agh memory search` JSON includes `filename`, `scope`,
    `workspace`, `type`, `name`, `mod_time` per
    `SearchResult` (`internal/memory/types.go:67-78`).
  - Agent's reply explicitly mentions `Lucia v3` (the workspace
    project preference); reviewer confirms.
evidence:
  - Search JSON output.
  - Recall block from prompt trace.
  - Agent reply + reviewer judgment.
failure_signatures:
  - More than 3 entries injected: budget violated.
  - Workspace project entry not in top-3: ranking regression
    (`memory.recall-precedence`).
  - Provenance fields missing in JSON: catalog projector regression.
cleanup:
  - Delete all four seeds.
```

### MEM-07 — Memory provenance survives consolidation (compaction never erases source)

```yaml qa-scenario
id: mem-07-provenance-survives-consolidation
title: Provenance row in memory_operation_log persists across one consolidation run; agent can still answer "when was this written and by whom?"
theme: memory.provenance
coverage:
  primary:
    - memory.provenance-survives-consolidation
    - consolidation.fires-once
  secondary:
    - consolidation.gate-time
    - consolidation.diff-auditable
risk: high
live: true
provider: real-claude-code
preconditions:
  - One global `user` memory seeded by an agent at T0 (capture
    `memory.write` row id from `memory_operation_log`).
  - `[memory.dream]` enabled with low thresholds so a consolidation
    cycle fires at T1.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/catalog.go:78
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/catalog.go:549
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/dream.go:217
steps:
  - Snapshot `memory_operation_log` and the memory file's content +
    SHA256 at T0.
  - Trigger consolidation; wait for completion.
  - Snapshot at T1.
  - Drive a real Claude Code session: "When was the memory titled
    `<name>` written?"
expected:
  - `memory_operation_log` at T1 still contains the original
    `memory.write` row from T0 (id, timestamp, agent_name unchanged).
    Consolidation may add MORE rows (a `memory.write` for the
    dream's compaction product, a `memory.search` row, a
    `memory.reindex` row), but the original is preserved.
  - The original memory file's body either matches T0 exactly OR
    has been replaced by a consolidation product whose
    `memory_operation_log` references the prior id (provenance
    chain visible).
  - Real Claude Code reply correctly cites the original timestamp
    (or admits uncertainty referencing the operation log) — i.e.,
    the agent has access to provenance through the operation log
    surface.
evidence:
  - T0 + T1 `memory_operation_log` snapshots (diff appended).
  - T0 + T1 file content + SHA256.
  - Agent reply.
failure_signatures:
  - T0's `memory.write` row missing at T1: provenance erasure;
    catalog event log truncated by consolidation. Critical bug.
  - File content changed without a corresponding new write row:
    consolidation wrote out-of-band. Bug.
cleanup:
  - Reset config; stop daemon.
```

### MEM-08 — Agent soul + per-agent memory directory: write/update/delete via authoring service; MEMORY.md cap holds

```yaml qa-scenario
id: mem-08-soul-and-agent-memory
title: SOUL.md authored via managed authoring; per-agent persistence under <workspace>/.agh/agents/<agent>/SOUL.md; MEMORY.md index never grows beyond 200 lines / 25_000 bytes
theme: soul.authoring
coverage:
  primary:
    - soul.allowed-fields
    - soul.path-isolation
    - soul.digest-stability
    - soul.managed-only-mutation
    - memory.index-cap
  secondary:
    - soul.compact-projection
    - memory.atomic-write
risk: critical
live: true
provider: real-claude-code
preconditions:
  - Workspace `wsp-mem08` with one agent `coder` defined in
    `<workspace>/.agh/agents/coder/AGENT.md`.
  - Soul config defaults: `agents.soul.enabled=true`,
    `agents.soul.max_body_bytes=32768`,
    `agents.soul.context_projection_bytes=2048`.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/soul/soul.go:159
  - /Users/pedronauck/Dev/compozy/agh/internal/soul/soul.go:483
  - /Users/pedronauck/Dev/compozy/agh/internal/soul/soul.go:590
  - /Users/pedronauck/Dev/compozy/agh/internal/soul/persistence.go:43
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/authored_context.go:103
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/store.go:280
steps:
  - Author a valid `SOUL.md` body file off-tree:
    `printf "%s\n" "---" \
     "version: 1" "role: implementer" "tone: [direct, concise]" \
     "principles: [explain why before what]" \
     "memory_policy: [prefer feedback memories over project notes]" \
     "---" "I am the coder agent. I implement, I do not pontificate." \
     > /tmp/soul-coder.md`
  - Write via authoring service:
    `agh agent soul write coder --file /tmp/soul-coder.md
      --workspace wsp-mem08 -o json`
    Capture the returned `digest`, `revision_id`.
  - Verify file landed at
    `<wsp-mem08>/.agh/agents/coder/SOUL.md` (canonical persisted
    location per `_techspec_soul.md` storage section).
  - Try a forbidden field write — replace `tone` with `tools: [read]`
    and re-write. Expect the authoring service to reject with a
    `forbidden_field` diagnostic (per `internal/soul/soul.go:704-726`).
  - Try a path-escape write — pass `--file ../../../etc/passwd`.
    Expect rejection with `path_escape` diagnostic.
  - Update via a second valid revision (change `role` to
    `senior-implementer`); confirm `agh agent soul history coder
     --workspace wsp-mem08 -o json` shows two revisions, both
    `action=put`, distinct `new_digest`.
  - Roll back to revision 1:
    `agh agent soul rollback coder --revision-id <rev1-id>
     --workspace wsp-mem08 -o json`.
    Confirm the third row in history has `action=rollback`.
  - Delete:
    `agh agent soul delete coder --expected-digest <current-digest>
     --workspace wsp-mem08 -o json`.
    Confirm the file is gone and the fourth history row is
    `action=delete` with `new_digest=""`.
  - In parallel, write 250 distinct memories to the workspace memory
    directory (one per second, real Claude Code agent in a loop)
    so the workspace `MEMORY.md` index would naturally exceed 200
    lines. Verify the prompt-injected index is bounded: drive one
    more session and inspect the prompt trace; the `## Workspace
    MEMORY.md Index` block must be ≤200 lines and ≤25_000 bytes,
    with a `_Index truncated to fit prompt limits._` line when
    truncated (per `internal/memory/assembler.go:148-159`).
expected:
  - Valid write succeeds. `agent_soul_snapshots` has one row with
    `digest`, `profile_json`, `body`. `agent_soul_revisions` has
    one `action=put` row.
  - Forbidden-field write fails with diagnostic `forbidden_field`,
    snapshot/revision NOT inserted (managed-only mutation honors
    validation).
  - Path-escape write fails with diagnostic `path_escape`, no
    file created at `/etc/passwd` (obviously) or anywhere else
    outside the workspace root.
  - Update + rollback + delete all complete; history shows the
    full chain `[put, put, rollback, delete]`.
  - `digestPrefix="agh.soul.v1\n"` digest stable: writing the same
    body twice yields the same digest (verified via repeat write
    after rollback).
  - 250-memory write loop: every write succeeds, the on-disk
    `MEMORY.md` may be 250 lines but the prompt-injected index is
    capped per `defaultIndexLines=200` and `defaultIndexBytes=25_000`
    (`internal/memory/store.go:24-25`). Daemon log emits "memory:
    truncated memory index".
evidence:
  - `agent_soul_snapshots` + `agent_soul_revisions` rows for the run.
  - `agh agent soul history coder` JSON output across the four
    operations.
  - Prompt trace for the post-loop session showing the truncated
    index block.
  - File listing of `<wsp-mem08>/.agh/agents/coder/` at each step.
failure_signatures:
  - Forbidden-field write succeeds: `soul.allowed-fields` violated;
    critical authority leak.
  - Path-escape write succeeds: filesystem escape; critical
    security bug.
  - Digest changes for identical body: `soul.digest-stability`
    violated (likely a non-canonical JSON marshaling regression in
    `internal/soul/soul.go:483-490`).
  - Prompt index >200 lines: `memory.index-cap` violated.
  - Rollback returns 200 OK but no third history row appears:
    revision append silently dropped.
cleanup:
  - Restore `<wsp-mem08>/.agh/agents/coder/` (delete coder's SOUL.md
    + revisions).
  - Bulk-delete the 250 seeded memories
    (`for f in <wsp-mem08>/.agh/memory/*.md; do
       agh memory delete "$f" --scope workspace --workspace wsp-mem08;
     done`).
```

### MEM-09 — Lifecycle hook on_session_created order + fail-open

```yaml qa-scenario
id: mem-09-lifecycle-session-created
title: session.pre_create + session.post_create hooks run in hierarchy + alphabetical order; one hook fails non-required → subsequent hooks still run; required failure halts
theme: lifecycle.session-pre-create
coverage:
  primary:
    - lifecycle.session-pre-create
    - lifecycle.fail-open
    - lifecycle.required-fail-closed
  secondary:
    - hooks.ordering
risk: high
live: true
provider: real-claude-code
preconditions:
  - Three hook declarations registered at `session.pre_create`
    (mode=sync) in the workspace `wsp-mem09`:
    - `a-memory-warmup` (subprocess that succeeds; `required=false`)
    - `b-memory-warmup` (subprocess that exits 1; `required=false`)
    - `c-memory-warmup` (subprocess that succeeds; `required=true`)
  - A workspace-level skill is the source of all three.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/events.go:54
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/pipeline.go:59
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/pipeline.go:71
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/ordering.go
steps:
  - Variant 1 (default): `agh sessions start --agent claude-code
     --workspace wsp-mem09 --prompt "list files"`.
    Capture hook traces and final session outcome.
  - Variant 2: promote `b-memory-warmup` to `required=true`,
    re-create the session.
expected:
  - Variant 1: hook trace shows execution order
    `a-memory-warmup → b-memory-warmup → c-memory-warmup`
    (alphabetical at the same precedence layer per
    `internal/hooks/ordering.go`).
    `b` outcome=`failed` (subprocess exit 1), but `c` still runs
    and outcome=`applied`. Session creation succeeds.
  - Variant 2: hook trace ends at `b`'s failure with
    `outcome=failed`, dispatch halts with wrapped error
    `hooks: required hook %q failed for event %q: %w`
    (per `internal/hooks/pipeline.go:71-77`). `c` never fires.
    Session creation aborts with that error in the daemon response.
evidence:
  - Daemon log + EventStore trace ordering for both variants.
  - Final session row state (created vs not created).
failure_signatures:
  - Variant 1 halts at `b`: fail-open broken for non-required
    hooks. Critical regression — the lifecycle is supposed to
    proceed.
  - Variant 1 hook order is `b → a → c` or any non-alphabetical
    permutation: ordering invariant broken.
  - Variant 2 proceeds past `b`: required-fail-closed bypass.
cleanup:
  - Remove the three seeded hooks.
```

### MEM-10 — Lifecycle hook timeout (5s default) is fail-open

```yaml qa-scenario
id: mem-10-lifecycle-timeout-fail-open
title: A 30s on_session_created subprocess hook hits the 5s default timeout; fail-open semantics — error logged, session proceeds
theme: lifecycle.fail-open
coverage:
  primary:
    - lifecycle.fail-open
    - hooks.timeout-fail-open
  secondary:
    - lifecycle.session-pre-create
risk: medium
live: true
provider: real-claude-code
preconditions:
  - One hook at `session.pre_create` whose subprocess body is
    `sleep 30 && echo '{}'`, registered with default
    `timeout` (5s — `internal/hooks/executor_subprocess.go:23`)
    and `required=false`.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/executor_subprocess.go:23
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/executor_subprocess.go:224
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/pipeline.go:71
steps:
  - Start session: `agh sessions start --agent claude-code
     --workspace wsp-mem10 --prompt "ping"`.
  - Capture hook trace + daemon log + session row state.
expected:
  - Hook run record `outcome=failed`, error wraps
    `context.DeadlineExceeded` (verifiable via the dispatch chain
    at `internal/hooks/pipeline.go:71-77`).
  - Session is still created and proceeds to its first turn.
  - Daemon log explicitly mentions the timeout.
evidence:
  - Hook run record from EventStore.
  - Daemon log fragment.
  - `sessions` row showing session reached `running` state.
failure_signatures:
  - Session blocks for 30s: timeout not enforced (executor regression).
  - Session aborts with the hook error: fail-open broken for
    non-required hooks.
cleanup:
  - Remove seeded hook.
```

### MEM-11 — Memory write at workspace scope from inside an agent does not leak to global

```yaml qa-scenario
id: mem-11-scope-isolation-no-leak
title: Agent writes a `project` memory; writes default to workspace scope; the memory is invisible at global scope; collision with a same-named global memory is permitted (different keys)
theme: memory.scope.dual
coverage:
  primary:
    - memory.scope.dual
    - memory.default-scope
    - memory.workspace-isolation
  secondary:
    - memory.cli-and-agent-parity
risk: high
live: true
provider: real-claude-code
preconditions:
  - Workspace `wsp-mem11`.
  - Pre-seeded GLOBAL memory `auth-policy.md`
    (frontmatter `type: user`).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/types.go:237
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/store.go:117
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/store.go:506
steps:
  - Drive Claude Code: `--prompt "Save a project memory describing
     this workspace's auth policy: 'OAuth2 + PKCE only; no JWT.'
     Use the memory write tool with type=project."`
  - Verify via CLI: the memory landed at workspace scope:
    `agh memory list --scope workspace --workspace wsp-mem11`
    contains a new `type: project` row with the body.
  - Verify it does NOT appear at global scope:
    `agh memory list --scope global` shows only the seeded
    `auth-policy.md`, not the new project memory.
  - Try the inverse: from the agent, request global write — the
    agent must use the explicit `--scope global` flag (or the
    Host API equivalent). Verify the agent cannot
    accidentally cross scopes by simply writing the same filename.
expected:
  - Workspace write lands at
    `<wsp-mem11>/.agh/memory/<filename>.md` per
    `pathFor` (`internal/memory/store.go:532`).
  - Global scope listing is unchanged.
  - When the agent writes `auth-policy.md` at global scope (same
    filename as the seed), the existing seed is overwritten
    (atomic replace) — but a write at workspace scope with the
    same filename creates a separate file in the workspace
    directory; both coexist.
evidence:
  - Two `agh memory list` outputs (global + workspace).
  - On-disk inspection of both directories.
  - `memory_operation_log` rows for the writes.
failure_signatures:
  - Workspace write appears in global scope: scope leakage.
  - Global write inadvertently mutates the workspace file:
    cross-scope accidental write.
cleanup:
  - Restore the seeded global memory; delete the workspace
    memory.
```

### MEM-12 — Cross-workspace isolation: real Claude Code in B cannot see A's memory

```yaml qa-scenario
id: mem-12-cross-workspace-isolation
title: Workspace A has a workspace-scoped fact; a real Claude Code session in workspace B cannot recall it
theme: memory.workspace-isolation
coverage:
  primary:
    - memory.workspace-isolation
    - memory.scope.dual
  secondary:
    - memory.recall-precedence
risk: critical
live: true
provider: real-claude-code
preconditions:
  - Workspace `wsp-mem12-A` with one workspace-scoped `project`
    memory: body "The deploy-key for staging is rotated every
    Wednesday; current rotation = 0xDEADBEEF-2026-04."
  - Workspace `wsp-mem12-B` with no memory at all.
  - No global memory mentioning "deploy-key".
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/store.go:506
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/catalog.go:733
steps:
  - Drive Claude Code in workspace B:
    `agh sessions start --agent claude-code --workspace wsp-mem12-B
     --prompt "What is the current staging deploy-key rotation?"`.
  - Capture the agent's reply.
  - Independently probe the catalog for B:
    `agh memory search "deploy-key" --workspace wsp-mem12-B
     -o json`.
  - Cross-check by running the same prompt in A — the agent must
    answer correctly there.
expected:
  - In B: agent replies "I do not have that information" or
    similar; reviewer judges the reply does NOT include the secret
    rotation token. The `agh memory search` JSON returns zero
    rows.
  - In A: agent replies with the rotation token; the same search
    returns one row.
  - No global memory was written in A (verify via
    `agh memory list --scope global`).
evidence:
  - Both transcripts (A + B).
  - Both `agh memory search` JSON outputs.
  - Forbidden-needle audit on B's transcript:
    `rg -n '0xDEADBEEF-2026-04' <B-transcript>` returns zero.
failure_signatures:
  - B's reply contains the rotation token: cross-workspace leak —
    critical security violation.
  - B's `agh memory search` returns the row: catalog scope filter
    broken (`internal/memory/catalog.go:733`).
cleanup:
  - Delete A's seeded memory.
```

### MEM-13 — Stale-memory verification: agent flags freshness warning

```yaml qa-scenario
id: mem-13-stale-memory-flagged
title: A 7-day-old memory referencing a renamed file path surfaces with a freshness warning; agent reply prompts user to verify
theme: memory.staleness
coverage:
  primary:
    - memory.staleness
    - memory.recall-bounded
  secondary:
    - memory.recall-precedence
risk: medium
live: true
provider: real-claude-code
preconditions:
  - Workspace `wsp-mem13`.
  - Seeded memory `wsp-mem13/.agh/memory/legacy-runtime.md` with
    body "The runtime entry point is at `cmd/agh-old/main.go`."
    Backdate via `touch -d '7 days ago' <path>` so its mtime is
    7 days old.
  - The actual file `cmd/agh-old/main.go` does NOT exist (the
    runtime moved to `cmd/agh/main.go`).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/staleness.go:28
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/recall.go:78
steps:
  - Drive Claude Code: `--prompt "Where is the runtime entry
     point for AGH?"`.
  - Capture the daemon prompt trace and the agent reply.
expected:
  - Recall block includes the `legacy-runtime.md` entry with a
    `Freshness:` warning line of the form "This memory is 7 days
    old. Verify against current state before asserting as fact."
    (per `internal/memory/staleness.go:30-35`).
  - Agent's reply either (a) cites `cmd/agh/main.go` after
    verifying via filesystem read, OR (b) flags the staleness
    explicitly and asks for verification. EITHER outcome is a
    pass — the load-bearing assertion is the freshness warning,
    not the agent's exact behavior.
evidence:
  - Prompt trace showing the freshness warning.
  - Agent transcript.
failure_signatures:
  - Freshness warning absent: `staleness` not wired into recall
    augmenter (regression in `internal/memory/recall.go:78`).
  - Agent confidently cites the stale path without any caveat:
    if AND ONLY IF the freshness warning is also missing — count
    as failure. If the warning is present, this is a real-LLM
    judgment call → reviewer marks `follow-up`.
cleanup:
  - Delete the stale memory.
```

### MEM-14 — DELETE / UPDATE flow: explicit forget + next-session non-recall

```yaml qa-scenario
id: mem-14-explicit-forget-flow
title: User says "forget X"; auto-memory rule removes the entry; next-session recall does not surface it
theme: delete-update.flow
coverage:
  primary:
    - delete-update.flow
    - memory.cli-and-agent-parity
  secondary:
    - memory.recall-bounded
    - memory.provenance-survives-consolidation
risk: high
live: true
provider: real-claude-code
preconditions:
  - Workspace `wsp-mem14`.
  - One global `user` memory `editor-preference.md` body "I prefer
    Vim with vim-fugitive and vim-go."
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/store.go:174
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/catalog.go:64
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/store.go:836
steps:
  - Session 1: `--prompt "I switched to Helix; please forget my
     Vim preference."`
    Capture turn-by-turn tool calls (the agent should call the
    memory delete tool / `memory.delete`).
  - Verify file removed:
    `test ! -f ~/.agh/memory/editor-preference.md`.
  - Verify catalog row removed:
    `agh memory search "Vim preference" -o json --limit 5` returns
    zero rows that reference the deleted entry.
  - Verify operation log retains the delete record:
    `agh memory history --operation memory.delete --since 5m`
    shows one row with `filename=editor-preference.md`.
  - Session 2: `--prompt "What editor do I use?"`. Capture reply.
expected:
  - File deleted on disk.
  - Catalog FTS row removed via the AFTER DELETE trigger
    (`internal/memory/catalog.go:64-67`).
  - `memory_operation_log` has a `memory.delete` row referencing
    `editor-preference.md`. Note: the corresponding original
    `memory.write` row from when the file was first authored
    REMAINS in the log (provenance trail preserved per MEM-07).
  - Session 2 recall does NOT inject the deleted entry. Agent
    replies along the lines of "I do not have that information"
    or asks the user.
evidence:
  - File listing before/after.
  - `agh memory search` + `agh memory history` JSON outputs.
  - Session 2 prompt trace + transcript.
failure_signatures:
  - File still present: `Store.Delete` regression.
  - Catalog row still present: AFTER DELETE trigger not firing.
  - Operation log shows no `memory.delete` row: event recording
    broken.
  - Session 2 recall surfaces the deleted memory: cache leakage
    in catalog.
cleanup:
  - None.
```

### MEM-15 — Type taxonomy compliance: reference + feedback structure

```yaml qa-scenario
id: mem-15-taxonomy-compliance
title: A `reference` memory is saved with pointer-style content; a `feedback` memory carries Why/How-to-apply structure; lint asserts taxonomy compliance
theme: memory.taxonomy
coverage:
  primary:
    - memory.taxonomy
    - taxonomy.lint
  secondary:
    - memory.atomic-write
    - memory.cli-and-agent-parity
risk: medium
live: true
provider: real-claude-code
preconditions:
  - Workspace `wsp-mem15`.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/store.go:149
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/types.go:14
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/assembler.go:17
steps:
  - Reference: drive `--prompt "Save a reference memory: 'Lucia v3
     getting-started: https://lucia-auth.com/main/getting-started'"`.
    Capture the file body.
  - Feedback: drive `--prompt "Save a feedback memory about my
     last code review: I confused 'capability' with 'recipe' in
     a docs PR; remember to use only 'capability' going forward."
     Use the structured Why / How-to-apply layout."
    Capture the file body.
  - Run a taxonomy lint pass (manual):
    - For the `reference` body, assert it contains a URL or a
      file path reference. (Convention.)
    - For the `feedback` body, assert it contains both "Why" and
      "How to apply" markers (or recognizable equivalents like
      `# Why` / `# How to apply`).
expected:
  - Both files write successfully and pass `Header.Validate`.
  - Reference body contains a URL or file path. If not, the
    scenario records `outcome=follow-up` with a note that
    runtime taxonomy lint is currently advisory (no enforcement
    layer in `Store.Write`); see §3 gap 1.
  - Feedback body has Why + How-to-apply structure. Same
    follow-up handling if absent — surface gap to the
    implementing agent.
evidence:
  - Both file bodies.
  - `memory_catalog_entries` rows for both.
  - Lint check output (markdown checklist).
failure_signatures:
  - Either file fails to write despite valid frontmatter:
    `Store.Write` regression.
  - The runtime ENFORCES Why/How-to-apply structure and the
    agent's free-form prose is rejected: documented expectation
    diverges from runtime behavior — surface the lint surface to
    the implementing agent.
cleanup:
  - Delete both seeded memories.
```

### MEM-16 — Real consolidation diff: before/after MEMORY.md auditable snapshot

```yaml qa-scenario
id: mem-16-consolidation-diff-audit
title: Capture before/after MEMORY.md from soul / workspace memory across one consolidation run; diff is reproducible and explained by the operation log
theme: consolidation.diff-auditable
coverage:
  primary:
    - consolidation.diff-auditable
    - memory.provenance-survives-consolidation
  secondary:
    - consolidation.fires-once
    - consolidation.gate-time
risk: medium
live: true
provider: real-claude-code
preconditions:
  - Workspace `wsp-mem16` with 6 small workspace memories
    (mixed `project` + `reference`) seeded across the prior
    "session" period (each backed by a real completed Claude
    Code session so the sessions gate also passes).
  - `[memory.dream]` config: `min_hours=0`, `min_sessions=5`
    (exact, so we know this fires).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/store.go:280
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/dream.go:217
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/consolidation/runtime.go:250
steps:
  - Snapshot T0:
    - `<wsp-mem16>/.agh/memory/MEMORY.md` content.
    - All 6 memory file bodies + their SHA256s.
    - `memory_operation_log` rows.
  - Trigger consolidation:
    `agh memory consolidate --workspace wsp-mem16 --wait`.
  - Wait for the spawned consolidation session to complete
    (`agh sessions list --type memory-consolidation`).
  - Snapshot T1: same set as T0.
  - Compute diffs (`diff T0/MEMORY.md T1/MEMORY.md`,
    `diff T0/<file>.md T1/<file>.md`).
expected:
  - T1 `MEMORY.md` differs from T0 (consolidation produced at
    least one merged or summarized memory).
  - Every difference is explained by an operation-log row
    between T0 and T1 (`memory.write` for new/merged files,
    `memory.delete` for any removed files).
  - The combined diff is auditable: a reviewer can reconstruct
    why each file changed without consulting the model's
    transcript.
  - Lock mtime advanced; subsequent ticks log "time gate blocked
    consolidation".
evidence:
  - T0 + T1 directory snapshots (tarball under
    `mem-16-snapshots/`).
  - Diff output.
  - `memory_operation_log` rows for the window.
  - Consolidation session transcript.
failure_signatures:
  - T1 == T0 (no changes): consolidation no-op despite gates
    passing — verify the spawn actually invoked the agent
    (`internal/memory/consolidation/runtime.go:261-282`).
  - Diff includes changes with no matching operation-log row:
    out-of-band write — provenance trail broken.
  - Lock mtime not advanced after spawn: success path missed
    `Release` (`internal/memory/lock.go:122-129`).
cleanup:
  - Restore lock; reset workspace memory directory.
```

### MEM-17 — High-write-rate stress (1k writes/sec from a stress agent)

```yaml qa-scenario
id: mem-17-high-write-rate-stress
title: 1000 concurrent memory writes/sec for 10s; no corruption of MEMORY.md or FTS5 index; gates remain consistent
theme: high-write-rate
coverage:
  primary:
    - high-write-rate
    - memory.atomic-write
  secondary:
    - memory.index-cap
    - consolidation.gate-lock
risk: medium
live: false
provider: mock-acp
preconditions:
  - Workspace `wsp-mem17`.
  - A small stress driver (Go test binary or shell+xargs+`agh
     memory write`) that emits 10_000 unique frontmatter-valid
    memories over ≈10s.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/store.go:149
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/store.go:546
  - /Users/pedronauck/Dev/compozy/agh/internal/store/globaldb
steps:
  - Run the driver against the daemon. Cap target at ~1000 ops/sec
    via `xargs -P 16` or equivalent.
  - During the run, run `agh memory list --scope workspace
     --workspace wsp-mem17` periodically. Run a recall query
    every second.
  - After completion, run `agh memory search "stress-test-marker"
     --workspace wsp-mem17 -o json --limit 50` to confirm the
    catalog is consistent.
  - Run `PRAGMA integrity_check;` against `agh.db`.
expected:
  - All writes succeed (or any failure is a typed
    `ErrValidation` and the test driver records it).
  - `MEMORY.md` is well-formed (no torn lines, no zero-byte file
    visible mid-flight outside the atomic-rename window).
  - FTS5 index is queryable throughout the run; recall returns
    consistent rows for the stress marker.
  - `PRAGMA integrity_check;` reports `ok`.
  - No `consolidate-lock` corruption: lock file readable at all
    times (mtime + content).
evidence:
  - Driver log (count + per-second rate).
  - Periodic `agh memory list` snapshots.
  - Final `PRAGMA integrity_check` result.
  - Daemon log (filtered to memory subsystem).
failure_signatures:
  - Any `database is locked` error returned to the driver as a
    raw 500: locking primitive regression — should serialize
    transparently or return typed retry signal.
  - `PRAGMA integrity_check` not OK: corruption.
  - `MEMORY.md` content malformed mid-write: atomic-write
    primitive broken (`fileutil.AtomicWriteFile`).
cleanup:
  - Bulk-delete all seeded memories. Stop daemon. Archive
    catalog DB before reset.
```

## 8. Optional / nice-to-have scenarios (run if time)

### MEM-18 — Real OpenClaw cross-driver sanity for consolidation

```yaml qa-scenario
id: mem-18-cross-driver-consolidation
title: Re-run MEM-04 + MEM-16 with OpenClaw as the dream agent driver to prove the gate cascade is driver-agnostic
theme: consolidation.driver-parity
coverage:
  primary:
    - consolidation.gate-time
    - consolidation.gate-sessions
    - consolidation.diff-auditable
  secondary:
    - memory.provenance-survives-consolidation
risk: low
live: true
provider: real-openclaw
preconditions:
  - `[roles.dream]` config with `agent="openclaw"`.
  - Same seeds as MEM-16.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/consolidation/runtime.go:250
steps:
  - Re-run MEM-16 steps with the OpenClaw driver.
expected:
  - Identical outcome shape: `MEMORY.md` diff present,
    operation-log explanation complete, lock mtime advances.
  - Only the consolidation session's `agent_driver` differs.
evidence:
  - Same set as MEM-16, suffixed `-openclaw`.
failure_signatures:
  - OpenClaw driver fails where Claude Code passes: the
    consolidation contract leaked driver-specific assumptions.
cleanup:
  - Same as MEM-16.
```

### MEM-19 — Soul + memory together: prompt assembly contains soul block, memory index, recall block

```yaml qa-scenario
id: mem-19-prompt-composition
title: A real Claude Code session with both an active SOUL.md and seeded memories produces a prompt that contains all three blocks in deterministic order
theme: soul.compact-projection
coverage:
  primary:
    - soul.compact-projection
    - memory.index-cap
  secondary:
    - memory.recall-bounded
risk: low
live: true
provider: real-claude-code
preconditions:
  - Workspace `wsp-mem19` with one valid `SOUL.md` (per MEM-08
    happy path) and 3 seeded workspace memories.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/memory/assembler.go:97
  - /Users/pedronauck/Dev/compozy/agh/internal/situation/service.go:64
  - /Users/pedronauck/Dev/compozy/agh/internal/soul/soul.go:492
steps:
  - Drive a session with a non-trivial prompt; capture the prompt
    trace.
expected:
  - Prompt trace contains, in order: persona/soul block (compact
    projection), memory taxonomy + global index + workspace
    index, recall block (only when relevant), then the user
    message.
  - Compact soul projection ≤ `agents.soul.context_projection_bytes`
    (default 2048).
  - Memory index ≤ 200 lines / 25_000 bytes.
evidence:
  - Annotated prompt trace.
failure_signatures:
  - Order is jumbled: composition root assembled the prompt out
    of order (regression in
    `internal/memory/assembler.go:97-117`).
  - Soul body (full prose) appears in compact projection: budget
    not enforced.
cleanup:
  - Delete soul + memories.
```

## 9. Coverage matrix (this child)

| Coverage ID                                | Scenarios                                                |
| ------------------------------------------ | -------------------------------------------------------- |
| `memory.taxonomy`                          | MEM-01, MEM-06, MEM-15                                   |
| `memory.scope.dual`                        | MEM-02, MEM-11, MEM-12                                   |
| `memory.default-scope`                     | MEM-01, MEM-11                                           |
| `memory.atomic-write`                      | MEM-01, MEM-02, MEM-08, MEM-15, MEM-17                   |
| `memory.index-cap`                         | MEM-08, MEM-17, MEM-19                                   |
| `memory.recall-bounded`                    | MEM-01, MEM-06, MEM-13, MEM-19                           |
| `memory.recall-precedence`                 | MEM-02, MEM-06, MEM-12                                   |
| `memory.staleness`                         | MEM-01, MEM-13                                           |
| `memory.provenance-survives-consolidation` | MEM-06, MEM-07, MEM-14, MEM-16, MEM-18                   |
| `memory.workspace-isolation`               | MEM-11, MEM-12                                           |
| `memory.cli-and-agent-parity`              | MEM-01, MEM-02, MEM-04, MEM-11, MEM-14, MEM-15           |
| `consolidation.gate-time`                  | MEM-03, MEM-07, MEM-16, MEM-18                           |
| `consolidation.gate-sessions`              | MEM-04, MEM-18                                           |
| `consolidation.gate-lock`                  | MEM-03, MEM-05, MEM-17                                   |
| `consolidation.gate-order`                 | MEM-03, MEM-04                                           |
| `consolidation.fires-once`                 | MEM-03, MEM-05, MEM-07, MEM-16                           |
| `consolidation.lock-rollback`              | MEM-05                                                   |
| `consolidation.diff-auditable`             | MEM-07, MEM-16, MEM-18                                   |
| `soul.allowed-fields`                      | MEM-08                                                   |
| `soul.forbidden-overlap`                   | MEM-08                                                   |
| `soul.path-isolation`                      | MEM-08                                                   |
| `soul.digest-stability`                    | MEM-08                                                   |
| `soul.compact-projection`                  | MEM-08, MEM-19                                           |
| `soul.managed-only-mutation`               | MEM-08                                                   |
| `lifecycle.session-pre-create`             | MEM-09, MEM-10                                           |
| `lifecycle.session-post-stop`              | MEM-09 (variant 1 cleanup-pass)                          |
| `lifecycle.fail-open`                      | MEM-09, MEM-10                                           |
| `lifecycle.required-fail-closed`           | MEM-09                                                   |
| `delete-update.flow`                       | MEM-14                                                   |
| `taxonomy.lint`                            | MEM-15                                                   |
| `high-write-rate`                          | MEM-17                                                   |
| `agent-scope-isolation` (gap)              | MEM-08 (per-agent dir), MEM-13 (gap surface)             |

Total: 17 mandatory + 2 optional = 19 scenarios. Every coverage ID is
exercised by at least one scenario; load-bearing IDs by at least two.

## 10. Forbidden-needle list (transcript and event payloads)

Per the openclaw `forbiddenNeedles` pattern. None of the following may
appear in any outbound message, transcript, SSE event, or audit log
across any MEM scenario:

- Any provider API key shape: `sk-`, `xoxb-`, `AKIA`, `ya29.`.
- Any raw `agh_claim_<>=12 random char>` (regex
  `agh_claim_[A-Za-z0-9_-]{12,}`) — same redaction discipline as the
  autonomy kernel child.
- Any cross-workspace fact in a session bound to a different workspace
  (MEM-12 enforces this with the literal seed `0xDEADBEEF-2026-04`).
- Raw SOUL.md body (full prose) injected into `/agent/context`'s
  compact projection (MEM-08, MEM-19).
- The deleted legacy `recipe`/`workflow`/`procedure`/`playbook`
  vocabulary (per `docs/_memory/glossary.md` — canonical term is
  `capability`).

A single scenario test failure on this list is shippability-critical
and must be triaged immediately.

## 11. Reporting contract

Each scenario writes the four-artifact set required by the openclaw
operator-flow pattern (markdown report + JSON summary + observed events
+ combined log). The aggregate `mem-summary.json` for this child carries
the coverage matrix from §9 alongside per-scenario `outcome ∈ {worked,
failed, blocked, follow-up}` and machine-readable timing.

The scenario operator runs in-character (per the `real-scenario-qa`
skill); every run ends with a Worked / Failed / Blocked / Follow-up
section covering all 17 mandatory scenarios. A child run is shippable
only when:

- Every mandatory scenario is `worked` or has an explicit accepted
  follow-up (the documented gaps in §3 may be accepted as follow-up
  rather than failure if and only if the implementing agent
  acknowledges them in writing in `mem-summary.json`).
- MEM-05 + MEM-12 are both clean (the consolidation lock gate and the
  cross-workspace isolation tests are non-negotiable).
- No forbidden-needle hit anywhere.
- `make verify` passed on the SUT branch before this child ran (cite
  commit SHA in `mem-summary.json`).
