## 0.2.10 - 2026-06-18

### ♻️  Refactoring

- Tui redesign (#201)
### 🎉 Features

- Worktree-backed parallel multi-run for tasks run --multiple (#200)- Add Devin CLI agent support (#204)
### 🐛 Bug Fixes

- Reviews watch bug
### 📚 Documentation

- Release notes
### 📦 Build System

- Skeeper config (#206)- Converge skeeper sidecar lock to main branch

### Release Notes

#### Features

##### Devin CLI agent support
Compozy now supports [Devin CLI](https://devin.ai/cli) as a first-class ACP execution runtime, alongside Claude Code, Codex, Copilot, Cursor, Droid, OpenCode, and the others.

### Usage

Install Devin CLI and expose `devin` on your `PATH`, then select it like any other runtime:

```bash
compozy tasks run my-feature --ide devin
```

Compozy launches it via `devin acp`. Skill installation (`compozy setup`) and the runtime registry both recognize `devin`, so it shows up in agent detection and the setup catalog.

### Notes

- Devin CLI resolves its own model, reasoning, and access defaults, so Compozy does not pass model/reasoning/access bootstrap flags to it.
- The default model recorded for the runtime is `anthropic/claude-opus-4-6`.

##### Parallel multi-run with git worktree isolation
`compozy tasks run --multiple` can now execute the batched task workflows **in parallel**, each in its own isolated git worktree, instead of running them one at a time. This builds on the `--multiple` queue introduced in 0.2.8 and is fully additive — the default behavior (enqueued, one-at-a-time) is unchanged.

### Starting a parallel run

```bash
# Run children concurrently, each in its own worktree
compozy tasks run --multiple alpha,beta --parallel

# Bound how many children start at once (default 2)
compozy tasks run --multiple alpha,beta --parallel --parallel-limit 3
```

### Configuration

The mode and fanout limit can be set per workspace under `[tasks.run]`:

| Key                           | Flag override      | Default    | Meaning                                         |
| ----------------------------- | ------------------ | ---------- | ----------------------------------------------- |
| `run_multiple_mode`           | `--parallel`       | `enqueued` | `enqueued` or `parallel`                        |
| `run_multiple_parallel_limit` | `--parallel-limit` | `2`        | Max children started concurrently (must be > 0) |

`--parallel` and `--parallel-limit` are valid only with `--multiple`, and each flag overrides the corresponding config value for that invocation. Passing `--parallel-limit` while the resolved mode is `enqueued` is now rejected rather than silently ignored.

### Behavior

- **Worktree isolation.** Each child run gets a dedicated git worktree so concurrent agents never collide on the working tree. Worktree paths include a SHA-256 digest of the parent run id, so distinct parent runs can't collide on the same slug.
- **Bounded fanout with fail-late aggregation.** Children start up to the parallel limit at a time; a failing child no longer aborts its siblings — failures are aggregated and reported after the batch settles.
- **Worktree handoff surfaced.** The TUI and CLI render each child's worktree path so you know exactly where each run executed.
- **Contract aligned.** Multi-run item status now reports `running` (matching the daemon and `docs/events.md`) instead of the stale `active` that leaked into the OpenAPI schema and generated TypeScript client; a contract test now pins the runtime constants to the published enum so they can't drift again.

##### Steer running agents — pause, message, and resume
You can now interrupt a running agent mid-task and steer it without killing the run. While a job is active in the run TUI, press `p` to pause it. Pausing happens at a safe boundary between ACP prompt turns — the current turn finishes, then the job holds.

### How it works

- **Pause (`p`).** The timeline help advertises the shortcut whenever the focused job is pausable. Pausing emits `job.pausing` then `job.paused` and parks the job between turns instead of cancelling it.
- **Message and resume.** Once paused, a composer opens (`Message paused task`). Type guidance for the agent — up to 64KB — and send it. The job resumes with your message as the next user turn, emitting `job.resumed` with the new message ID.
- **No lost context.** The agent keeps its session; your message is folded into the existing conversation rather than starting over.

### Under the hood

The daemon exposes new run-job control endpoints (`PauseRunJob`, `SendRunJobMessage`) and three new event kinds — `job.pausing`, `job.paused`, `job.resumed` — published to the run journal and the live event stream. The public OpenAPI schema and generated TypeScript client were regenerated to cover the new control surface, so external consumers can drive pause/message programmatically.

#### Fixes

##### Reviews watch reliability and clearer ACP setup failures
`compozy reviews watch` could fail a remediation round on a transient agent startup stall, and the resulting error gave little to go on. This release fixes the reliability gap and makes ACP session-setup failures diagnosable across every command, not just reviews watch.

### What was wrong

- A slow ACP session setup (creating, loading, or setting the mode of a session) that hit the inactivity timeout was treated as a hard, non-retryable failure — even though an inactivity stall is a transport/runtime hiccup, not a protocol rejection.
- Setup failures surfaced as opaque errors: no launch command, no agent stderr, and no indication when the real cause was a context cancellation (e.g. `Ctrl+C` or a parent timeout).
- Reviews watch child runs did not consistently honor the project's configured retry policy.

### What changed

- **Setup-stage inactivity timeouts are now retryable.** A stalled session create/load/set-mode is handled as a timeout and retried, instead of failing the job outright.
- **Richer setup diagnostics.** When ACP session setup fails, the error now includes the launch command, the agent's stderr, and — when the context was cancelled — the underlying cancellation cause joined into the error chain. This applies to all ACP runtimes.
- **Child retries honor project config.** Reviews-watch child runs now pass through whether the project configured `max_retries`, so the watch loop respects the workspace retry policy rather than guessing.

#### Highlights

##### Redesigned run TUI
The interactive terminal UI for `compozy tasks run`, `compozy exec`, and `compozy reviews watch` has been rebuilt from the ground up. The wizard (workspace/agent/model selection and validation) and the live execution view now share a single, consistent layout system with a redesigned sidebar, timeline, and run summary.

### What changed

- **Wizard flow.** Workspace, runtime, and validation steps were reworked for clearer state, tighter spacing, and more legible status. The validation form and run summary render with the same theme as the execution view.
- **Execution layout.** The sidebar, timeline, and summary panes were redesigned for denser, more readable run state — per-job status, attempts, and streamed ACP activity are easier to follow at a glance.
- **Multi-run tabs.** The `--multiple` tab strip was polished so the brand renders once, the spinner survives idle/active tab switches, and an idle tab no longer leaves a dangling spinner.

### Why it matters

This is a visual and structural overhaul of the entire run experience. Existing commands and flags are unchanged — only the presentation and interaction model improved. It also lays the groundwork for the new interactive job control described in the "Steer running agents" note.