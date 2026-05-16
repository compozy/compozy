## 0.2.5 - 2026-05-16

### 🎉 Features

- Add zsh task completion plugin docs and script (#149)
### 📚 Documentation

- Add star history on readme

## 0.2.4 - 2026-05-14

### 🐛 Bug Fixes

- Codex acp integration (#151)

## 0.2.3 - 2026-05-09

### 🐛 Bug Fixes

- Cwd path

## 0.2.2 - 2026-05-08

### 🎉 Features

- Add qa extension (#138)
### 🐛 Bug Fixes

- Workspace register (#140)- Workspace discover path- Prevent false task completion via prompt kickoff + worktree diff-check (#144) (#145)
### 📚 Documentation

- Update- Release notes
### 📦 Build System

- Release tool

### Release Notes

#### Features

##### Force-confirmation when archiving non-terminal workflows
Archiving a workflow that still has open work no longer silently succeeds. The daemon now returns a typed `workflow_force_required` error when the target workflow has non-terminal tasks or unresolved review issues, and the dashboard surfaces it as an inline confirmation dialog so you can either resolve the open items first or explicitly archive with `force = true`.

### What changed

- `internal/core/archive.go` introduces `ErrWorkflowForceRequired` and a structured `WorkflowArchiveForceRequiredError` that reports task and review counts:

  ```go
  type WorkflowArchiveForceRequiredError struct {
      WorkspaceID      string
      WorkflowID       string
      Slug             string
      Reason           string
      TaskTotal        int
      TaskNonTerminal  int
      ReviewTotal      int
      ReviewUnresolved int
  }
  ```

- The daemon HTTP API maps that error to `code: "workflow_force_required"` with a 409 response, so frontends can detect it without parsing strings.
- `model.ArchiveConfig.Force` and the kernel `WorkflowArchiveCommand.Force` field now flow end-to-end, so a retry with `force=true` bypasses the gate.
- The web archive flow (`web/src/routes/_app/workflows.tsx` + `web/src/systems/workflows/adapters/workflows-api.ts`) catches the typed error, opens an alert dialog with task/review counts, and re-issues the archive call with `force: true` if you confirm.

### Web UI

A new `AlertDialog` primitive in `@compozy/ui` powers the confirmation. The flow is:

1. Click _Archive_ on a workflow with open tasks or reviews.
2. The daemon returns `workflow_force_required` with counts (e.g. `task_non_terminal: 2`, `review_unresolved: 1`).
3. The UI opens a confirmation dialog explaining what will be archived anyway.
4. Confirm → the same archive request is retried with `force: true`; the response includes `forced: true` and the counts that were overridden.

### API shape

```jsonc
// Without force, when state is open:
HTTP 409
{
  "code": "workflow_force_required",
  "message": "workflow \"my-feature\" requires force archive confirmation: ...",
  "details": {
    "task_total": 5,
    "task_non_terminal": 2,
    "review_total": 4,
    "review_unresolved": 1
  }
}

// Retry with force = true:
{
  "archived": true,
  "forced": true,
  "completed_tasks": 5,
  "resolved_review_issues": 4
}
```

Workflows whose state is already clean continue to archive on the first call with no prompt — the gate only fires when there is genuinely open work.

##### Built-in QA workflow extension
Compozy now ships a built-in `cy-qa-workflow` extension that automatically attaches QA-planning and QA-execution tasks to any PRD-driven workflow, with curated runtimes per task. The extension lives at `extensions/cy-qa-workflow/` and follows the same on-disk contract as user extensions, so it can be customized or replaced project-by-project.

When enabled, every `compozy tasks run <slug>` over a PRD-mode workflow ends up with two extra tasks at the tail of `_tasks.md`:

| Task                                                   | Purpose                                                                                                                                | Type   | Complexity |
| ------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------- | ------ | ---------- |
| `<Workflow> QA plan and regression artifacts`          | Generates feature-level test plans, execution-ready test cases, and regression suites under `.compozy/tasks/<workflow>/qa/`            | `docs` | `high`     |
| `<Workflow> QA execution and operator-flow validation` | Executes the generated plan, files bug reports for confirmed failures, fixes root causes, and finishes only after `make verify` passes | `test` | `critical` |

The execution task depends on the report task; the report task depends on every other implementation task in the workflow, so QA always runs last.

### Curated runtimes

The extension also pins per-task runtimes via the new `plan.pre_resolve_task_runtime` hook so each QA task runs on the IDE/model best suited to it — no manual `--task-runtime` needed:

| QA task      | IDE      | Model     | Reasoning effort |
| ------------ | -------- | --------- | ---------------- |
| QA report    | `claude` | `opus`    | `xhigh`          |
| QA execution | `codex`  | `gpt-5.5` | `xhigh`          |

Override on a per-run basis with `--task-runtime`, or per-project via `[[tasks.run.task_runtime_rules]]`.

### Prompt augmentation

`cy-qa-workflow` also patches the agent session at create time:

- The QA execution prompt is prefixed with `/goal …` so the agent enters goal-driven mode and only finishes after `make verify` passes.
- The QA report prompt sets `CLAUDE_CODE_EFFORT_LEVEL=xhigh` in the session env to lift Claude's effort ceiling for plan generation.

### Manifest

```toml
# extensions/cy-qa-workflow/extension.toml
[extension]
name = "cy-qa-workflow"
version = "0.1.0"
description = "Adds Compozy QA report and QA execution tasks to workflow runs"
min_compozy_version = "0.1.10"

[subprocess]
command = "go"
args = ["run", "."]

[security]
capabilities = ["plan.mutate", "agent.mutate", "tasks.read", "tasks.create"]

[[hooks]]
event = "plan.pre_discover"
required = true

[[hooks]]
event = "plan.pre_resolve_task_runtime"
required = true

[[hooks]]
event = "agent.pre_session_create"
required = true
```

### Idempotency

- Tasks are detected by HTML markers (`<!-- compozy-qa-workflow:qa-report -->` / `<!-- compozy-qa-workflow:qa-execution -->`) plus title/type heuristics, so re-running the workflow does not duplicate them.
- `update_index = true` is set on the new `host.tasks.create` request, so the entries appear in `_tasks.md` in the right order on first run.

### SDK additions used by the extension

- `TaskCreateRequest.UpdateIndex` (`update_index` in JSON / TS) — when `true`, the host appends the created task to `_tasks.md`. Documented in `docs/extensibility/host-api-reference.md`.
- `TaskFrontmatter.Dependencies` — extensions can now seed task dependencies directly when creating a task.
- `SessionRequest` / `ResumeSessionRequest` now use a stable readable JSON contract (prompts are plain strings, not base64), matching the runtime-side ACP contract used by hook payloads and patches.

#### Fixes

##### Workspace register/resolve path fixes
Two long-standing workspace-discovery papercuts are fixed. `compozy workspaces register` and `resolve` now accept relative paths the same way every other Compozy command does, and workspace auto-discovery no longer treats the home-scoped `~/.compozy/` runtime directory as a project-local workspace marker.

Closes #139.

### Relative paths now work for `register` / `resolve`

Before, the API client sent paths through unchanged after `strings.TrimSpace`. A relative path like `.` or `./my-project` was forwarded to the daemon as-is, where it resolved against the daemon's working directory instead of the caller's, producing confusing "workspace not found" errors or registering the wrong directory.

The client now normalizes the argument before sending it:

```go
// internal/api/client/operator.go
func normalizeWorkspacePathArg(path string) (string, error) {
    trimmed := strings.TrimSpace(path)
    if trimmed == "" {
        return "", nil
    }
    if filepath.IsAbs(trimmed) {
        return filepath.Clean(trimmed), nil
    }
    absolutePath, err := filepath.Abs(trimmed)
    if err != nil {
        return "", fmt.Errorf("resolve workspace path %q: %w", path, err)
    }
    return filepath.Clean(absolutePath), nil
}
```

This normalization runs for both `RegisterWorkspace` and `ResolveWorkspace`, so:

```bash
cd ~/code/my-feature
compozy workspaces register .            # now registers /Users/you/code/my-feature
compozy workspaces resolve ./sub-project  # resolves against the caller's CWD
```

### `~/.compozy/` is no longer auto-detected as a workspace

`discoverWorkspaceRootFromStart` walks up the filesystem looking for a `.compozy/` marker directory. When `compozy` was invoked from anywhere under `$HOME` that did not contain its own `.compozy/`, the walk would eventually find `~/.compozy/` — the home-scoped daemon runtime root — and register the user's home directory (or some ancestor) as a workspace.

The discovery loop now resolves the global Compozy marker once and skips it during the walk, so only project-local `.compozy/` directories are treated as workspace roots:

```go
// internal/core/workspace/config.go
globalMarkerDir, hasGlobalMarker := discoverGlobalWorkspaceMarkerDir()
// ...
if err == nil && info.IsDir() {
    // The home-scoped Compozy directory stores global runtime/config state.
    // It must not redefine arbitrary paths under HOME as local workspaces.
    if !hasGlobalMarker || !sameWorkspaceMarkerDir(candidate, globalMarkerDir) {
        return current, nil
    }
}
```

Comparison is symlink-aware (`filepath.EvalSymlinks` on both sides), so installs that symlink `~/.compozy/` are still correctly excluded.

### Coverage

New tests pin the behavior end-to-end:

- `internal/api/client/client_transport_test.go` — relative paths are normalized before transport.
- `internal/cli/operator_commands_integration_test.go` — `register` / `resolve` from a relative CWD produce absolute paths in the registry.
- `internal/core/workspace/config_test.go` — discovery skips `~/.compozy/` even when started from `$HOME`.
- `internal/store/globaldb/registry_test.go` — registry insert/lookup is consistent with the normalized paths.

## 0.2.1 - 2026-05-01

### 🐛 Bug Fixes

- Binary release

## 0.2.0 - 2026-05-01

### Refactoring

- Daemon improvements (#121)

### Features

- Add optional sound notifications on run lifecycle events (#96)
- Global config defaults (#106)
- Add per task prop selection (#109)
- Migrate to daemon — **BREAKING:** (#112)
- Daemon web UI (#122)
- Web UI polish (#125)
- Review watch (#133)

### Bug fixes

- Daemon adjustments (#116)
- Harden runtime activity and version handling (#127)
- Release adjustments (#131)
- Infer task type during migrate (#129)
- Watch adjustments
- Lint errors

### Documentation

- Release notes
- Daemon PRD
- New PRDs
- Updates
- Add release notes

### CI/CD

- Fix auto-docs
- Add release notes
- Fix Windows

### 🧪 Testing

### Release Notes

#### Breaking Changes

##### Daemon-based architecture

Compozy now runs every task, review, and exec workflow through a long-lived, home-scoped daemon at `~/.compozy/`. The daemon owns runtime state in SQLite (`~/.compozy/db/global.db` plus per-run `run.db`), exposes a UDS + HTTP API, and supports re-attach and observe across separate CLI invocations. Existing scripts that called `compozy start` or `compozy fix-reviews` need updates — most legacy top-level commands are gone or moved under the new `tasks`, `reviews`, `runs`, and `daemon` groups. The CLI auto-starts the daemon on first invocation, so most users do not need to start it explicitly.

### What's new

- `compozy daemon start | status | stop` lifecycle commands, plus `--foreground` for attached runs and `--web-dev-proxy <url>` for proxying a frontend dev origin through the daemon HTTP transport.
- `compozy workspaces list | show | register | unregister | resolve` workspace registry — workspaces are registered lazily on first use, or explicitly via `register`.
- `compozy tasks run <slug>` daemon-backed workflow runner with `--attach auto|ui|stream|detach`, `--ui`, `--stream`, `--detach`, and `--task-runtime` overrides.
- `compozy reviews fetch | list | show | fix` review command family (replaces the old top-level `fix-reviews` / `fetch-reviews`).
- `compozy runs attach <run-id> | watch <run-id> | purge` for re-attaching to live runs and pruning terminal artifacts.
- `--format text|json` flag on operator/daemon commands for machine-readable output.
- New durable stores: `~/.compozy/db/global.db` (workspaces, runs index) and per-run `~/.compozy/runs/<run-id>/run.db`.
- `compozy migrate` is now **required** before daemon-backed commands run on legacy projects (it also infers task types — see the dedicated note).

### Breaking changes

| Area                    | Before                                   | After                                                                                           |
| ----------------------- | ---------------------------------------- | ----------------------------------------------------------------------------------------------- |
| Workflow run            | `compozy start --name <slug>`            | `compozy tasks run <slug>` (top-level `start` is removed)                                       |
| Review fix              | `compozy fix-reviews`                    | `compozy reviews fix` (top-level `fix-reviews` kept as alias)                                   |
| Review fetch            | `compozy fetch-reviews`                  | `compozy reviews fetch` (top-level `fetch-reviews` kept as alias)                               |
| Per-task runtime config | `[[start.task_runtime_rules]]`           | `[[tasks.run.task_runtime_rules]]` (TOML), or `--task-runtime` (CLI/TUI)                        |
| Runtime artifacts       | `<workspace>/.compozy/runs/<run-id>/`    | `~/.compozy/runs/<run-id>/` (now includes durable `run.db`)                                     |
| Sync semantics          | `compozy sync` regenerated `_meta.md`    | Reconciles workflow state into `global.db`; one-time cleanup of legacy `_meta.md` / `_tasks.md` |
| Preflight               | `compozy start` skill check              | `tasks run` and `reviews fix` block on missing skill installs                                   |
| Public Go API           | File-based readers in `pkg/compozy/runs` | Daemon-transport readers; signature changes in `Run`, `watch`, `tail`, `replay`                 |
| Migrate                 | Recommended                              | **Required** before any daemon-backed workflow command on legacy projects                       |

### New daemon workflow

```bash
# Lifecycle (most users do not need explicit start; tasks/reviews auto-start)
compozy daemon start                                     # detached, returns status
compozy daemon start --foreground                        # attached
compozy daemon start --foreground \
  --web-dev-proxy http://127.0.0.1:3000                  # for UI development
compozy daemon status --format json
compozy daemon stop --force                              # cancel runs, then stop

# Workspaces
compozy workspaces register .
compozy workspaces list --format json
compozy workspaces show <id-or-path>

# Run a workflow
compozy tasks run user-auth                              # auto-attach (TUI if interactive)
compozy tasks run user-auth --stream                     # textual stream
compozy tasks run user-auth --detach                     # fire-and-forget
compozy tasks run user-auth \
  --task-runtime type=frontend,ide=codex,model=gpt-5.5

# Reattach / observe / purge
compozy runs attach <run-id>
compozy runs watch  <run-id>
compozy runs purge

# Reviews
compozy reviews fetch user-auth --provider coderabbit --pr 42
compozy reviews list  user-auth
compozy reviews fix   user-auth --ide claude --concurrent 2 --batch-size 3
```

### Daemon lifecycle improvements

- `daemon start --foreground` runs the daemon attached to the current shell with structured logs.
- HTTP port defaults to OS-chosen (ephemeral) and is reported by `daemon status`. Pin it with `COMPOZY_DAEMON_HTTP_PORT=<n>`. Bind host is loopback-only (`127.0.0.1`) and non-loopback origins are rejected at the middleware layer.
- Attaching to a run that has already settled now falls back to streaming the persisted event log instead of erroring.
- `daemon stop` accepts `--force` to cancel owned runs before shutdown; otherwise it drains gracefully.

### Migration steps

1. Upgrade the binary. On first invocation the daemon creates `~/.compozy/{config.toml,agents,extensions,state,daemon,db,runs,logs,cache}`.
2. For legacy projects with XML-tagged artifacts: run `compozy migrate` once before any daemon-backed command.
3. Replace scripts:
   - `compozy start --name X` → `compozy tasks run X`
   - `compozy fix-reviews` → `compozy reviews fix` (alias still works)
   - `compozy fetch-reviews` → `compozy reviews fetch` (alias still works)
4. Update TOML: rename `[[start.task_runtime_rules]]` to `[[tasks.run.task_runtime_rules]]`. Move `id=` selectors to `--task-runtime` (TOML rejects `id=` rules).
5. Stop reading `<workspace>/.compozy/runs/` directly — runtime artifacts now live in `~/.compozy/runs/<run-id>/` and include a durable `run.db`. Use `pkg/compozy/runs` (daemon transport) or the daemon HTTP/UDS API.
6. Optional: set `COMPOZY_DAEMON_HTTP_PORT=<n>` to pin the HTTP port (`0` requests an ephemeral port). `COMPOZY_WEB_DEV_PROXY` mirrors `--web-dev-proxy`.

#### Features

##### Global config defaults

Set personal defaults once in `~/.compozy/config.toml` and have them apply across every project. Project-level `.compozy/config.toml` always takes precedence, so teams keep control while individuals stop repeating themselves.

### Example

```toml
# ~/.compozy/config.toml  (global — applies to all projects)
[defaults]
ide = "claude"
model = "sonnet"
access_mode = "default"
auto_commit = true

[sound]
enabled = true
on_completed = "glass"
on_failed = "basso"

[exec]
model = "gpt-5.5"
verbose = true
```

```toml
# .compozy/config.toml  (project — overrides global)
[defaults]
model = "o4-mini"

[start]
include_completed = true
```

With both files above the effective config resolves to:

| Field                     | Value       | Source       |
| ------------------------- | ----------- | ------------ |
| `defaults.ide`            | `"claude"`  | global       |
| `defaults.model`          | `"o4-mini"` | project wins |
| `defaults.auto_commit`    | `true`      | global       |
| `sound.enabled`           | `true`      | global       |
| `exec.model`              | `"gpt-5.5"` | global       |
| `start.include_completed` | `true`      | project      |

All sections supported in project config (`[defaults]`, `[start]`, `[exec]`, `[fix_reviews]`, `[fetch_reviews]`, `[tasks]`, `[sound]`) work in the global file with the same schema.

##### Optional sound notifications on run lifecycle events

Opt-in audio cues that play when a run completes or fails, so you can step away from long-running sessions without missing the result. Ships **disabled by default** — no sound unless you explicitly enable it.

### Setup

Add a `[sound]` section to `.compozy/config.toml` (project or global):

```toml
[sound]
enabled = true
on_completed = "glass"   # plays on successful completion
on_failed = "basso"      # plays on failure or cancellation
```

### Built-in presets

Seven presets work cross-platform out of the box:

| Preset      | macOS                                   | Linux                                        | Windows                             |
| ----------- | --------------------------------------- | -------------------------------------------- | ----------------------------------- |
| `glass`     | `/System/Library/Sounds/Glass.aiff`     | `freedesktop/stereo/complete.oga`            | `Media\Windows Notify Calendar.wav` |
| `basso`     | `/System/Library/Sounds/Basso.aiff`     | `freedesktop/stereo/dialog-error.oga`        | `Media\chord.wav`                   |
| `ping`      | `/System/Library/Sounds/Ping.aiff`      | `freedesktop/stereo/message.oga`             | `Media\notify.wav`                  |
| `hero`      | `/System/Library/Sounds/Hero.aiff`      | `freedesktop/stereo/bell.oga`                | `Media\tada.wav`                    |
| `funk`      | `/System/Library/Sounds/Funk.aiff`      | `freedesktop/stereo/bell.oga`                | `Media\Ring06.wav`                  |
| `tink`      | `/System/Library/Sounds/Tink.aiff`      | `freedesktop/stereo/message.oga`             | `Media\ding.wav`                    |
| `submarine` | `/System/Library/Sounds/Submarine.aiff` | `freedesktop/stereo/phone-incoming-call.oga` | `Media\ringin.wav`                  |

### Custom sounds

Pass an absolute path to use your own audio file:

```toml
[sound]
enabled = true
on_completed = "/Users/you/sounds/success.wav"
on_failed = "/Users/you/sounds/fail.wav"
```

### Lifecycle events

| Event         | Config field   | When it fires                                  |
| ------------- | -------------- | ---------------------------------------------- |
| Run completed | `on_completed` | Task finishes successfully                     |
| Run failed    | `on_failed`    | Task errors out                                |
| Run cancelled | `on_failed`    | Task is interrupted (reuses the failure sound) |

Playback is synchronous with a 3-second timeout — a missing or slow audio file never blocks shutdown. Errors are logged at debug level and never surface to the user.

##### Per-task runtime overrides on tasks run

Pick a different IDE, model, or reasoning effort per task type — or per individual task — for a single `compozy tasks run` invocation, instead of running the whole batch on one global runtime. Selection is exposed three ways: a repeatable `--task-runtime` CLI flag, an interactive form on `tasks run`, and a `[[tasks.run.task_runtime_rules]]` TOML section. A new `plan.pre_resolve_task_runtime` extension hook lets extension authors resolve per-task runtime programmatically.

### CLI

`--task-runtime` is repeatable. Each value is a comma-separated rule with a selector (`id=` **or** `type=`) and at least one override (`ide=`, `model=`, `reasoning-effort=`).

```bash
# All frontend tasks → Codex with high reasoning, plus one task forced to xhigh
compozy tasks run multi-repo \
  --ide claude --model opus \
  --task-runtime "type=frontend,ide=codex,model=gpt-5.5,reasoning-effort=high" \
  --task-runtime "id=task_07,reasoning-effort=xhigh"
```

### TOML

Persistent type-scoped defaults live under `[[tasks.run.task_runtime_rules]]`. `id=` selectors are CLI/TUI-only by design — config rejects them.

```toml
[defaults]
ide = "codex"
model = "gpt-5.5"
reasoning_effort = "medium"

[[tasks.run.task_runtime_rules]]
type = "frontend"
model = "gpt-5.5"
reasoning_effort = "high"

[[tasks.run.task_runtime_rules]]
type = "docs"
ide = "claude"
model = "opus"
```

### Rule keys

| Key                                     | Where        | Description                                                                       |
| --------------------------------------- | ------------ | --------------------------------------------------------------------------------- |
| `id`                                    | CLI/TUI only | Match a single task by PRD task id                                                |
| `type`                                  | CLI/TUI/TOML | Match all tasks of this type (e.g. `frontend`, `docs`)                            |
| `ide`                                   | all          | `claude`, `codex`, `copilot`, `cursor-agent`, `droid`, `gemini`, `opencode`, `pi` |
| `model`                                 | all          | Any model accepted by the chosen IDE                                              |
| `reasoning-effort` / `reasoning_effort` | all          | `low`, `medium`, `high`, `xhigh`                                                  |

Each rule must have a selector and at least one override. Mixing `id` and `type` in a single rule is an error.

### Precedence (high → low at execution)

1. CLI/TUI `id=` rules
2. CLI/TUI `type=` rules
3. Config `[[tasks.run.task_runtime_rules]]` (type-only)
4. `[defaults]`

### Extension hook

Extension authors can resolve runtime programmatically via the new `plan.pre_resolve_task_runtime` hook (helper: `onPlanPreResolveTaskRuntime`). Later hooks (`plan.post_prepare_jobs`, `job.pre_execute`, `run.pre_start`) are now hard-guarded against runtime mutation for workflow runs — use the new hook instead.

##### Watch mode for PR review remediation

`compozy reviews watch` runs a long-lived loop that polls your review provider, fetches each new actionable round, runs `reviews fix`, optionally auto-pushes the resulting commits, and repeats until the PR is clean or a max-rounds cap is hit. The watch run shows up in the dashboard as a parent run with each round's `reviews fix` linked underneath, so you can step away from a noisy PR and come back to a finished branch.

### CLI

```bash
# Auto-push each round until clean (or max 6 rounds)
compozy reviews watch tools-registry --provider coderabbit --pr 85 \
  --auto-push --until-clean --max-rounds 6

# Follow events live instead of backgrounding
compozy reviews watch tools-registry --provider coderabbit --pr 85 --stream

# Tune timing
compozy reviews watch my-feature --provider coderabbit --pr 85 \
  --poll-interval 30s --review-timeout 30m --quiet-period 20s
```

`reviews watch` does not support cockpit UI attach — `--ui`, `--attach ui`, and `--tui` are rejected. Use `--stream` to follow events or `--detach` for fire-and-forget.

### TOML

```toml
[defaults]
auto_commit = true   # required when watch_reviews.auto_push = true

[fetch_reviews]
provider = "coderabbit"

[watch_reviews]
max_rounds     = 6
poll_interval  = "30s"
review_timeout = "30m"
quiet_period   = "20s"
auto_push      = true
until_clean    = true
push_remote    = "origin"
push_branch    = "feature/reviews"   # must be set together with push_remote
```

### How it works

1. Take a snapshot of git state and reconcile any already-committed unpushed commits (emitted as `round = 0` push events).
2. Poll the provider every `poll_interval` until the PR head is **settled** — for CodeRabbit, that means the latest commit status is `success`, not just any submitted review.
3. Wait `quiet_period` for in-flight review activity to drain, then re-check status.
4. If the next round has actionable issues, spawn a child `reviews fix` run, await its terminal state, and (with `--auto-push`) `git push <remote> HEAD:<branch>`.
5. Loop until `clean` (provider returns no actionable issues) or `max_rounds`.

Defaults: 6 rounds, 30 s poll, 30 m review timeout, 20 s quiet period.

### Auto-push safety rails

- Forces `auto_commit=true` on child runs; rejects `--auto-commit=false`.
- Only ever runs `git push <remote> HEAD:<branch>` — never `restore`, `reset`, `clean`, or branch switching.
- Reconciles existing unpushed commits at startup so a watch run never re-pushes work it didn't produce.
- Config-driven `auto_push=true` requires `defaults.auto_commit=true`.
- `push_remote` and `push_branch` must be set together (or both omitted to resolve upstream).

### Extension hooks

Four new hooks let extensions observe and gate the loop. Hooks can veto a round / push (`continue=false`/`push=false` + `stop_reason`) but cannot fake a clean state:

| Hook                      | Fires                                                  |
| ------------------------- | ------------------------------------------------------ |
| `review.watch_pre_round`  | Before each provider poll / fix round                  |
| `review.watch_post_round` | After a child `reviews fix` run reaches terminal state |
| `review.watch_pre_push`   | Before auto-push, with the resolved remote/branch      |
| `review.watch_finished`   | When the watch loop ends (clean, max-rounds, or error) |

### Caveats

- Provider support: **CodeRabbit only** for the settle-gating logic in this release; other providers are wired via the registry but settle behavior is CodeRabbit-specific.
- Provider auth still uses the existing fetch path (CodeRabbit token, GitHub PR access). Shorter `poll_interval` values increase pressure on those rate limits.
- Each watch run shows up in the dashboard with a persisted `parent_run_id`; the parent and the active child collapse into a single active row, full history retained.

#### Fixes

##### Harden runtime activity tracking and version handling

A bundle of reliability fixes across the update notifier, native Codex ACP runtime, ACP activity tracking, the extension SDK, and the build toolchain.

### What's fixed

- **Update notifier no longer prompts a "downgrade"** on git-describe builds. Pre-release suffixes like `-15-g834fec6` are stripped before semver comparison, so a binary built ahead of the latest tag stops nagging users to install the older release. Identifiers like `1.2.3-1-gamma` are preserved unless the suffix is a plausible short SHA.
- **Native Codex ACP runtime accepts `codex/<model>` aliases.** The provider prefix is stripped before `SetSessionModel`, fixing rejections for ChatGPT-account Codex sessions.
- **ACP activity stays "active" for the full lifecycle of a session update**, including nested or concurrent submissions. Previously the tracker could mark a session idle while in-flight work was still being submitted, dropping events on the floor.
- **Extension SDK publishes `initialized` state before sending the initialize response**, fixing a host-side race where `runs.start` could be rejected as "extension not initialized" immediately after handshake.
- **`BUN_VERSION` is now a minimum supported version, not an exact pin.** Error messaging updated to "or newer / at least", so contributors with a newer Bun release stop seeing spurious version errors.

### Before / after

```
# Before: a binary built between releases prompted a "downgrade" install
$ compozy --version
v0.1.12-15-g834fec6
$ compozy ...
Update available: 0.1.12 (you have v0.1.12-15-g834fec6)

# After: git-describe suffix is stripped; no spurious prompt
$ compozy ...
(no update notice)
```

##### Migrate now infers task type for legacy workflows

`compozy migrate` no longer emits `type: ""` for legacy `feature` / `feature implementation` tasks, which previously broke `compozy sync` on the migrated workflow. A valid v2 task type is now inferred from the legacy type, with `domain` used as a constrained fallback only when the direct remap is genuinely ambiguous. The unmapped-type follow-up prompt is now emitted only when inference is unsafe.

This release also tightens API error reporting: validation/parse failures from the daemon HTTP API now return `422 Unprocessable Entity` with cleaner messages instead of generic `500`, and the API core preserves original error identity so callers using `errors.Is` / `errors.As` get consistent results.

### Before / after

```bash
# Before — produced workflow with empty `type`, then failed on sync.
compozy migrate
compozy sync   # error: missing/invalid task type

# After — migrated workflow has a valid inferred type; sync succeeds.
compozy migrate
compozy sync   # ok
```

#### Highlights

##### Daemon Web UI

Compozy now ships a built-in web UI served straight from the daemon. Start the daemon and you get a single-binary, localhost-only dashboard for browsing workspaces, workflows, tasks, runs, reviews, and memory — with live SSE-backed run streaming, raw event diagnostics, a run transcript viewer, and skeletons / empty-states throughout. Frontend assets are embedded in the Go binary, so there is nothing extra to install. Contributors can point the daemon at a Vite dev server with `--web-dev-proxy`.

### What's in the UI

- **Dashboard** with workspace KPIs and a "Sync all workflows" action.
- **Workflows** inventory, per-workflow task board, and task detail page.
- **Workflow Spec** viewer (PRD / TechSpec / ADR markdown rendered inline).
- **Memory** index plus per-workflow memory view.
- **Reviews** index, per-round view, and issue detail pages.
- **Runs** list with workflow filter, plus run detail with live event stream.
- **Run Event Feed** — raw daemon events with in-memory event store, SSE snapshots, heartbeat, and overflow framing.
- **Run Transcript Panel** — full transcript view of agent turns and tool calls for any run.
- **Workspace picker** with onboarding shell and live workspace WebSocket sync.
- New shared UI primitives: `Alert`, `EmptyState`, `Markdown`, `Metric`, `Skeleton`, `StatusBadge`, plus button loading state and token refresh.

### Getting started

```bash
# Start the daemon (foreground for visibility); the UI is served at the daemon HTTP port
compozy daemon start --foreground

# Discover the URL
compozy daemon status        # prints "http_port: <N>"
open "http://127.0.0.1:<N>"

# UI contributors: proxy the daemon to a Vite dev server
compozy daemon start --foreground --web-dev-proxy http://127.0.0.1:3000
```

### Defaults & overrides

| Setting            | Default                     | Override                                           |
| ------------------ | --------------------------- | -------------------------------------------------- |
| Bind host          | `127.0.0.1` (loopback only) | hard-coded; non-loopback binds are rejected        |
| HTTP port          | OS-chosen (ephemeral)       | `COMPOZY_DAEMON_HTTP_PORT=<n>`                     |
| Frontend dev proxy | off (embedded `web/dist`)   | `--web-dev-proxy <url>` or `COMPOZY_WEB_DEV_PROXY` |

```bash
# Pin the daemon UI to a known port
export COMPOZY_DAEMON_HTTP_PORT=4444
export COMPOZY_WEB_DEV_PROXY=http://127.0.0.1:3000   # only for UI dev
compozy daemon start
```

### Security model

There is no login — the UI is loopback-only and the API enforces:

- Host header must match localhost; non-`127.0.0.1` binds are rejected.
- Origin validation against the bound host.
- Per-session CSRF cookie + header.
- `X-Compozy-Active-Workspace` header propagated by the SPA.
- Standard hardening headers via `securityHeadersMiddleware`, plus ETag/304 caching for the embedded static assets.