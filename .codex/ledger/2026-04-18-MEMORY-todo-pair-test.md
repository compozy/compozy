Goal (incl. success criteria):

- Validate the `smux-compozy-pairing` skill end-to-end by creating a fresh disposable Node.js todo API workspace, running the interactive tmux/TUI orchestration, generating the Compozy artifacts, and then observing the final `compozy start` run.
- Success requires a real temporary workspace outside the repository scope, real Codex and Claude TUIs inside tmux, generated artifacts under `.compozy/tasks/todo-api/`, and an observed `compozy start` execution attempt.

Constraints/Assumptions:

- Keep the temp project outside `/Users/pedronauck/Dev/compozy/looper` so it does not inherit this repository's local coding policy.
- Use vendored local copies of the required skills inside the temp workspace; do not rely on a shared symlinked skill tree.
- Use interactive TUIs only for Codex and Claude; do not use `codex exec`, `codex review`, `claude -p`, or `claude --print`.
- The workflow contract is: optional `cy-create-prd`, then `cy-create-techspec`, then `cy-create-tasks`, then `compozy start`.
- The live test uses an explicit tmux socket because the default server was not stable across shell calls.

Key decisions:

- Older temp workspaces were removed as the harness evolved.
- Previous active workspace `/tmp/compozy-smux-todo-api-20260418-210648` was explicitly deleted before recreating the fixture from scratch.
- Active workspace path: `/tmp/compozy-smux-todo-api-20260418-213020`.
- Feature slug under test: `todo-api`.
- Active tmux socket: `/tmp/compozy-smux-todo-api-20260418-213020/.tmux-smux.sock`.
- Active session name: `smux-pair-todo-api`.
- Pane targets:
  - orchestrator: `smux-pair-todo-api:0.0`
  - Codex: `smux-pair-todo-api:0.1`
  - Claude: `smux-pair-todo-api:0.2`
- Pane labels via `tmux-bridge`:
  - `todo-api-orchestrator`
  - `todo-api-codex`
  - `todo-api-claude`
- Claude runs with `--model opus --permission-mode bypassPermissions`.
- The new live session uses an autonomous Codex orchestrator pane instead of the earlier passive-shell harness.

State:

- In progress. The fresh replacement workspace is live and the new tmux session is stable; Codex is warming the local context before drafting the PRD and Claude is already online on `tmux-bridge`.

Done:

- Deleted the previous temporary todo-api workspace and removed the stale tmux session before recreating the harness.
- Confirmed only the new disposable Node.js fixture remains under `/tmp/compozy-smux-todo-api-*`.
- Created the fresh disposable Node.js fixture at `/tmp/compozy-smux-todo-api-20260418-213020`.
- Scaffolded the repo with `package.json`, `README.md`, `src/server.js`, `.gitignore`, a local `AGENTS.md`, and a local ledger.
- Initialized git in the temp workspace and created the initial commit `37fb742` (`chore: bootstrap todo-api fixture`).
- Vendored the required skills into the temp workspace, including `smux`, `smux-compozy-pairing`, `cy-create-prd`, `cy-create-techspec`, `cy-create-tasks`, `cy-execute-task`, `cy-final-verify`, `brainstorming`, `compozy`, and `cy-workflow-memory`.
- Bootstrapped the new live tmux session with three real panes: autonomous orchestrator inbox, Codex writer, and Claude reviewer.
- Confirmed `tmux-bridge doctor` passes on the live session.
- Put pane `%0` into a passive `cat` inbox so worker replies render as plain text instead of being executed by fish.
- Resolved the initial Codex trust prompt and delivered fresh boot prompts to both workers through `tmux-bridge`.
- Passed the user the exact tmux attach command for the new live session.
- Ran `make verify` in the main repository after the ledger/session refresh; it passed with `DONE 2385 tests, 1 skipped` and `All verification checks passed`.

Now:

- Monitor the new session while Codex finishes local context loading and begins the PRD flow.

Next:

- Carry the session through PRD completion.
- Carry the session through TechSpec completion.
- Carry the session through `cy-create-tasks` and validate the generated task set.
- Observe the final `compozy start` run and summarize the end-to-end outcome.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: whether the session will finish PRD/TechSpec/tasks without exposing a new orchestration defect beyond the already-fixed start-environment leak.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-18-MEMORY-todo-pair-test.md`
- `/tmp/compozy-smux-todo-api-20260418-213020/`
- `/tmp/compozy-smux-todo-api-20260418-213020/.codex/ledger/2026-04-18-MEMORY-todo-api-pairing.md`
- `/tmp/compozy-smux-todo-api-20260418-213020/.tmux-smux.sock`
- Commands:
  - `tmux -S /tmp/compozy-smux-todo-api-20260418-213020/.tmux-smux.sock attach -t smux-pair-todo-api`
  - `tmux -S /tmp/compozy-smux-todo-api-20260418-213020/.tmux-smux.sock capture-pane -t %0 -p -S -220`
  - `tmux -S /tmp/compozy-smux-todo-api-20260418-213020/.tmux-smux.sock capture-pane -t %1 -p -S -220`
  - `tmux -S /tmp/compozy-smux-todo-api-20260418-213020/.tmux-smux.sock capture-pane -t %2 -p -S -220`
  - `TMUX_BRIDGE_SOCKET=/tmp/compozy-smux-todo-api-20260418-213020/.tmux-smux.sock TMUX_PANE=%0 tmux-bridge ...`
