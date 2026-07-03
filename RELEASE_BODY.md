## 0.2.11 - 2026-07-03

### 🎉 Features

- Agentic runs (#212)- Simplify repo-level default setup overrides (#90)- Support COMPOZY_HOME env override for home directory (#216)
### 🐛 Bug Fixes

- Parallel execution (#217)- Specifying the model on ACP (#215)- Worktree management (#223)- Restore run TUI elapsed timer across retry, failure, cancel, and remote paths (#221)
### 📚 Documentation

- Update skills- Add v0.2.11 release notes

### Release Notes

#### Features

##### Agentic recovery for failed runs
Run-producing commands can now **automatically remediate and restart a failed run** with a dedicated recovery agent. When enabled, a failed run is handed to the recovery agent, which diagnoses and attempts a fix, then the run is restarted — up to a bounded number of attempts — instead of stopping at the first failure.

### Enabling recovery

Recovery is **off by default**. Turn it on per invocation:

```bash
# Enable agentic recovery for this run
compozy tasks run my-feature --recovery

# Pick the recovery runtime and bound the attempts
compozy tasks run my-feature --recovery \
  --recovery-ide codex --recovery-model gpt-5.5 \
  --recovery-reasoning high --recovery-max-attempts 2
```

Use `--no-recovery` to force it off for a single invocation even when the workspace enables it.

### Flags

| Flag                      | Default   | Meaning                                             |
| ------------------------- | --------- | --------------------------------------------------- |
| `--recovery`              | `false`   | Enable agentic recovery for failed runs             |
| `--no-recovery`           | `false`   | Disable recovery for this invocation                |
| `--recovery-ide`          | `codex`   | ACP runtime used by the recovery agent              |
| `--recovery-model`        | `gpt-5.5` | Model used by the recovery agent                    |
| `--recovery-reasoning`    | `medium`  | Reasoning effort (`low`, `medium`, `high`, `xhigh`) |
| `--recovery-max-attempts` | `1`       | Maximum remediation-and-restart cycles (1–3)        |

### Configuration

Set workspace defaults under `[recovery]`:

```toml
[recovery]
enabled = true
ide = "codex"
model = "gpt-5.5"
reasoning_effort = "medium"
max_attempts = 1
```

Recovery config is resolved fresh for each invocation and is **not** persisted into run or exec metadata; the flags above always override `[recovery]` for a single command.

##### Dependency-aware parallel task execution
`compozy tasks run <slug> --parallel-tasks` now executes the pending task files of a single PRD workflow **in dependency-aware waves** instead of one task at a time. Independent tasks in the same wave run concurrently, each in its own isolated git worktree, and dependent tasks wait for their prerequisites to finish.

### Starting a parallel-tasks run

```bash
# Run one workflow's tasks by dependency waves
compozy tasks run my-feature --parallel-tasks
```

`--parallel-tasks` targets a single workflow and cannot be combined with `--multiple` (that flag drives the separate multi-slug queue). It overrides the workspace `[tasks.run.parallel] enabled` value for one invocation.

### Task-graph manifest

Waves are computed from a `_tasks.md` task-graph manifest (`compozy.tasks/v2`) that describes the task nodes and their dependency edges. The manifest is the source of truth for which tasks may run together and which must wait, so ordering is explicit and reproducible rather than inferred at runtime.

### Configuration

Set defaults per workspace under `[tasks.run.parallel]`:

| Key                                      | Default | Meaning                                                                                    |
| ---------------------------------------- | ------- | ------------------------------------------------------------------------------------------ |
| `enabled`                                | `false` | Turn on dependency-aware parallel task execution                                           |
| `max_concurrency`                        | `4`     | Cap on concurrent task worktrees within a single wave                                      |
| `[tasks.run.parallel.conflict_resolver]` | —       | Agent (`ide`, `model`, `reasoning_effort`, `max_attempts`) used to resolve merge conflicts |

### Agentic conflict resolution

When concurrent task worktrees are squash-merged back and collide, a bounded **conflict-resolver agent** attempts to resolve the merge automatically before the run fails. You can override the resolver per invocation with the hidden `--parallel-conflict-resolver-ide`, `--parallel-conflict-resolver-model`, and `--parallel-conflict-resolver-reasoning` flags, or configure it under `[tasks.run.parallel.conflict_resolver]`.

### Notes

- The task-run wizard and CLI both understand parallel-tasks mode, and the run emits richer parallel plan/start and per-task failure events so the TUI and journal reflect wave progress.
- Worktree isolation means concurrent tasks never edit the same checkout; merges back to the workspace are serialized deterministically.

##### Isolated Compozy homes with COMPOZY_HOME
Compozy now honors a `COMPOZY_HOME` environment variable as an opt-in override for the home root that everything home-scoped resolves against. When set to a non-empty value, Compozy uses that path instead of the implicit `$HOME/.compozy`.

### Why

The daemon is a singleton per home: every workspace that talks to `~/.compozy/daemon/daemon.sock` serializes through one engine. Operators running several independent projects in parallel previously had no first-class way to isolate them (the workaround was a fragile "mirror home" of symlinks).

`COMPOZY_HOME` is the official escape hatch. Point one shell at one home and another shell at a different home, and each gets its **own daemon, socket, lock, state, and global database**:

```bash
COMPOZY_HOME=~/.compozy-projectA compozy tasks run feature-a
COMPOZY_HOME=~/.compozy-projectB compozy tasks run feature-b
```

### What it covers

The override is honored consistently across every home-scoped consumer, not just the daemon socket:

- Home path resolution and layout (`ResolveHomeDir` / `ResolveHomePaths` / `EnsureHomeLayout`) and daemon startup.
- Global config loading and global workspace-marker detection.
- Extension discovery and enablement.
- Global reusable-agent discovery.

`~` and `~/` prefixes inside `COMPOZY_HOME` are expanded against the current user's home, so `COMPOZY_HOME=~/alt-compozy` works. When the variable is unset or empty, behavior is unchanged (`$HOME/.compozy`).

### Scope

This delivers the isolation escape hatch; a dedicated CLI `--home` flag and true parallel runs across workspaces inside a single daemon are intentionally out of scope and can layer on top later.

##### Repo-level default overrides in compozy setup
`compozy setup` can now save repo-level runtime defaults for you, so you no longer have to hand-edit `.compozy/config.toml` to change the built-in defaults for a project.

### What changed

After skill installation, interactive setup offers an optional step to override the built-in runtime defaults and persist them to the workspace config:

```toml
[defaults]
ide = "..."
model = "..."
reasoning_effort = "..."
```

- The IDE is chosen from a dropdown of supported runtimes, and the recommended model comes from the runtime registry's per-IDE default.
- The override prompt defaults to **No** — opt out and you keep the built-in defaults with no config change.
- Defaults are only written after setup completes with zero failures, and existing config sections are preserved on write.

### Notes

- The write is workspace-scoped: it lands in the repo's `.compozy/config.toml`.
- `compozy setup --global` does not prompt for repo defaults and never modifies workspace config.
- If your repo already has a custom model, setup preserves it; if the current model still matches the previously recommended one, changing the IDE updates the recommendation to the new IDE's default.

#### Fixes

##### ACP runs consistently apply the selected model
ACP-backed runs now apply the selected model reliably for both newly created and resumed sessions. Previously the chosen model could fail to take effect on some session paths, so an agent could run against the wrong model.

### What changed

- The model is trimmed and validated, then **set before the first prompt turn** for both new and resumed runs, so the very first turn already uses the intended model.
- When switching the model on a session fails, Compozy now performs a best-effort session cleanup instead of leaving a half-configured session around, reducing spurious retry, cancellation, and timeout errors on subsequent turns.

This makes `--model` (and the resolved workspace default) behave consistently across every ACP runtime, whether a run is started fresh or resumed.

##### Run TUI elapsed timer restored across all terminal outcomes
The run TUI's elapsed **timer no longer disappears after a retried job succeeds**, and the fix now covers every terminal outcome — success, failure, and cancel — as well as remote tabs that attach after a job has already finished.

### What was wrong

The UI derived a job's elapsed time from a locally tracked start timestamp that the retry flow never seeded: retry attempts emit no fresh "job started" event, and a remote tab can bootstrap a job mid-retry. The authoritative duration the executor already computes was being discarded, so the timer showed blank on retry and roughly `00:00` on remote pre-attach.

### What changed

- The executor's authoritative job duration is now threaded into the UI and preferred over the locally tracked value, with the local start timestamp backfilled for coherence.
- Duration is carried on **all** terminal payloads — completed, failed, and cancelled — using a zero-guarded elapsed calculation (a job can be given up or canceled before any attempt starts).
- The remote run-job summary contract carries the duration too, with a start→terminal timestamp fallback for historical journals, so remote tabs that attach after completion show the correct elapsed time.

The result is a correct timer for every job across retry, failure, cancel, and remote-attach paths, with no persistence or schema migration required.

##### Safer worktree management for parallel runs
This release hardens the git-worktree machinery that backs parallel runs (`--parallel-tasks` and `--multiple --parallel`), closing several ways a run could start in an unsafe state or leave worktrees behind.

### Preflight guards

- **Detached HEAD is rejected** before the daemon is contacted, so a parallel run can't start from a checkout with no branch to base worktrees on.
- **Parallel runs inside a managed worktree are refused** up front, preventing nested parallel execution from colliding with the worktree it's already running in.

### Purge and cleanup

- `compozy` purge now reports **both** purged runs and purged worktrees, with safer containment checks so it only removes paths it actually owns.
- Worktree purge uses deferred handling that behaves correctly for nested active runs and missing workspace roots, and tracks produced-vs-pre-existing task worktree changes accurately.
- Terminal shutdown now also stops spawned child processes, so quitting a run doesn't leave orphaned agent processes running.

### Git environment isolation

Git subprocesses now **ignore inherited repository-scoped Git environment variables** (e.g. `GIT_DIR`, `GIT_WORK_TREE`). Previously an ambient Git env from the parent shell or a wrapping tool could redirect worktree operations at the wrong repository; each worktree command now runs against the repository Compozy intends.

### Also

- Bumped the Go toolchain patch version (1.26.3 → 1.26.4).