---
name: agh-worktree-isolation
description: Isolates AGH runtime state for concurrent worktrees and QA runs. Use when another agent or runtime may share the machine, or when creating a parallel worktree. Do not use for single-worktree or build-only work.
trigger: explicit
argument-hint: "[scenario-slug]"
---

# Worktree Isolation

Provision one owned runtime envelope for a concurrent worktree or non-bootstrap
test run. Production-like QA should prefer `agh-qa-bootstrap`, which adds a
manifest, provider homes, browser policy, and targeted teardown evidence.

Creating the parallel checkout itself is a separate step: `make worktree-new SLUG=<slug>` (→ `./scripts/worktree.sh`) adds a sibling worktree and bootstraps it (mise pins, `bun install` + skill/AGENTS.md symlinks; `BUILD=1`/`E2E=1` opt-ins); `make worktree-bootstrap` preps an existing checkout; `./scripts/worktree.sh rm <slug>` removes. Go caches are shared machine-wide — no per-worktree Go setup. This skill then isolates the RUNTIME state that agents inside that worktree use. `make verify` and the E2E lanes self-queue machine-wide (L-030); scoped lanes are capacity-bounded and need no lock.

## Required Inputs

- **scenario-slug** (optional): a short kebab-case slug used to name the AGH_HOME directory and tmux socket. Defaults to `agh-iso-<timestamp>`.

## Procedures

**Step 1: Confirm the Concurrency Branch**

1. Look for explicit signals in the user's request: parallel-worktree language, "another agent is running", "tem outro agent trabalhando", "QA in parallel", or invocation under a worktree path like `Compozy/_worktrees/<slug>/`.
2. If no concurrency signal is present and no parallel runtime is planned, stop; this skill does not apply.
3. Confirm the scenario-slug, defaulting to a timestamped slug when omitted.

*Done when:* the concurrent runtime/test branch and one unique scenario slug are explicit.

**Step 2: Allocate AGH_HOME**

1. Run the bootstrap/mutating helper `python3 .agents/skills/agh/agh-worktree-isolation/scripts/allocate-isolation.py --slug "<scenario-slug>" [--prefer-worktree]`. Pass `--prefer-worktree` only from an actual parallel worktree. The script:
   - Creates a unique `AGH_HOME` directory under `${TMPDIR:-/tmp}/agh-iso-<slug>-<random>/` OR uses the worktree-scoped `Compozy/_worktrees/<slug>/.agh/` when invoked from a worktree.
   - Picks a free TCP port on `127.0.0.1` for the daemon HTTP server.
   - Creates a unique UDS path under `AGH_HOME`.
   - Picks a unique tmux socket path under the AGH_HOME (e.g., `${AGH_HOME}/tmux-bridge.sock`).
2. The script prints export statements, including `AGH_ISOLATION_ROOT`, suitable for `eval "$(...)"`.

*Done when:* the allocator returns one non-default, owned root and unique HTTP/UDS/tmux addresses.

**Step 3: Source the Envelope**

1. Capture the exported variables: `AGH_ISOLATION_ROOT`, `AGH_HOME`, `AGH_HTTP_PORT`, `AGH_UDS_PATH`, `TMUX_BRIDGE_SOCKET`.
2. For shells: `eval "$(python3 .agents/skills/agh/agh-worktree-isolation/scripts/allocate-isolation.py --slug "<slug>" [--prefer-worktree])"`.
3. For Make/CI invocations: pass the variables as overrides to the daemon start command.
4. Confirm the daemon does NOT write to `~/.agh/` or default port 23230.

*Done when:* the current shell exposes the exact allocator output and no default runtime path is reachable by the planned command.

**Step 4: Verify Isolation Before Action**

1. Confirm `AGH_HOME` is non-default and writable.
2. Confirm the chosen ports are not already bound (re-pick if necessary).
3. Confirm the tmux socket path is non-default and not held by another process.
4. Print a one-line summary: `slug, AGH_HOME, http port, uds path, tmux socket`.

*Done when:* every address is free immediately before launch and the summary identifies the owned root.

**Step 5: Run the Isolated Scenario**

1. Hand off to the inner skill (`real-scenario-qa`, `qa-execution`, `make test-e2e-runtime`, etc.).
2. Prefer `agh-qa-bootstrap` for production-like local QA because its own allocator also provides provider homes, a manifest, browser policy, Web proxy env, and teardown evidence.
3. Inner skills inherit the env via the shell session. Do not re-allocate.

*Done when:* every child process uses the same envelope and none writes to another worktree or default home.

**Step 6: Cleanup (processes ALWAYS, files optionally)**

1. Process teardown is mandatory on every terminal path. Run the mutating teardown helper only for this run's envelope:
   `python3 .agents/skills/agh/agh-qa-bootstrap/scripts/teardown-qa-env.py --root "$AGH_ISOLATION_ROOT"`
2. Confirm the targeted teardown reports `TEARDOWN_ALL_CLEAN=true`. Survivors are a blocking failure.
3. The `AGH_HOME` directory is left in place for forensic inspection unless the caller explicitly requests `--purge` for a temporary envelope.
4. Never purge a worktree-scoped `.agh`; it belongs to the user's checkout.
5. Use `make qa-reap` or teardown `--all` only for an intentional machine-wide stale-lab recovery, never as normal cleanup while other runs may be active.

*Done when:* the owned envelope is clean, unrelated labs remain untouched, and any retained files contain no live process state.

## Error Handling

- **No free port available:** retry with a wider range. If still no luck, surface the busy ports and exit.
- **AGH_HOME path collision:** the script uses random suffixes; collision is essentially impossible. If it happens, retry once.
- **User invokes without concurrency signal but with `--force`:** apply isolation. Some users always want isolated runs.
- **Worktree-scoped path lacks write permission:** fall back to TMPDIR-scoped path with a logged warning.
- **Targeted teardown cannot prove ownership:** stop and report the root/PIDs; never widen normal cleanup to `--all`.
