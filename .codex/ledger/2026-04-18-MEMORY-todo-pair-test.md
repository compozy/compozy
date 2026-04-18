Goal (incl. success criteria):

- Validate the new `smux-compozy-pairing` skill end-to-end by creating a disposable Node.js todo API workspace, running the interactive tmux/TUI orchestration, generating a TechSpec and task set, and then executing `compozy start`.
- Success requires a real temporary workspace outside the repository scope, real Codex and Claude TUIs inside tmux, generated Compozy artifacts under `.compozy/tasks/todo-api/`, and an observed result from `compozy start`.

Constraints/Assumptions:

- Keep the temp project outside `/Users/pedronauck/Dev/compozy/looper` so it does not inherit this repository's `AGENTS.md` and `CLAUDE.md`.
- Use vendored local copies of the required skills inside the temp workspace; the original symlink approach was replaced because it made asset resolution brittle for the worker TUIs.
- Use interactive TUIs only for Codex and Claude; do not use `codex exec`, `codex review`, `claude -p`, or `claude --print`.
- The user corrected the workflow contract: PRD creation is optional and, when needed, must go through `cy-create-prd` before `cy-create-techspec`.
- The live test uses an explicit tmux socket because the default server was not stable across shell calls.

Key decisions:

- Temporary workspace path: `/tmp/compozy-smux-todo-api-20260418-194527`.
- Feature slug under test: `todo-api`.
- Active tmux socket: `/tmp/compozy-smux-todo-api-20260418-194527/.tmux-smux.sock`.
- Current session name: `smux-pair-todo-api`.
- Pane labels: orchestrator `todo-api-orchestrator`, Codex `todo-api-codex`, Claude `todo-api-claude`.
- Claude must run with `--permission-mode bypassPermissions` or it cannot use Bash and `tmux-bridge`.
- Add a small local `AGENTS.md` in the temp workspace so task execution has an explicit verification contract (`npm test` plus a start smoke check).
- Treat the raw-shell orchestrator pane as a test harness only. Real workflow messages must still travel through `tmux-bridge`, but during harness runs the pane may need to be parked on `cat` so incoming bridge text is not executed by `zsh`.

State:

- In progress.

Done:

- Read the relevant execution contract from `cy-execute-task`.
- Created the temp workspace root.
- Confirmed the installed `compozy` CLI exposes both `exec` and `start`.
- Scaffolded the disposable Node.js workspace with `package.json`, `README.md`, `src/server.js`, `.gitignore`, and a local `AGENTS.md`.
- Initialized git in the temp workspace.
- Replaced the original `.agents/skills` symlink with vendored local copies of the required skills, including `smux`, `smux-compozy-pairing`, `cy-create-prd`, `cy-create-techspec`, `cy-create-tasks`, `cy-execute-task`, `cy-final-verify`, `brainstorming`, `compozy`, and `cy-workflow-memory`.
- Corrected the temp workflow to respect the optional PRD gate instead of seeding `_prd.md` by hand.
- Launched the live tmux session and confirmed Codex and Claude are running as interactive TUIs.
- Relaunched Claude with `claude --model opus --permission-mode bypassPermissions` so it could actually use `tmux-bridge`.
- Drove the PRD phase to completion:
  - `.compozy/tasks/todo-api/_prd.md` now exists.
  - `.compozy/tasks/todo-api/adrs/adr-001.md` now exists.
  - The workflow locked the internal-validation audience, end-to-end diagnostic success signals, the minimal item shape, and the strict diagnostic fixture approach.
- Started the TechSpec phase through `/cy-create-techspec todo-api`.
- Confirmed the first TechSpec checkpoint was delivered over `tmux-bridge` and answered with option A: keep built-in `node:http` with minimal routing and JSON helpers.
- Confirmed the second TechSpec checkpoint was delivered over `tmux-bridge` and answered with option A: keep `PATCH /todos/:id` with partial updates for `title` and/or `completed`.
- Identified and fixed a skill-level process issue: decisions already constrained by PRD locks or ADRs should not be reopened as fresh A/B/C/D menus; they should be carried forward or asked as single-option confirmations.

Now:

- Wait for Codex to finish the remaining TechSpec clarification loop and draft `_techspec.md`.

Next:

- Approve or revise the TechSpec draft when Codex sends it.
- Continue into `/cy-create-tasks todo-api`.
- Validate the generated task set.
- Run `compozy start --name todo-api --ide codex --model gpt-5.4 --reasoning-effort xhigh` and observe the result.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: how many more TechSpec checkpoints Codex will surface before drafting.
- UNCONFIRMED: whether additional task-generation clarifications will be needed before `_tasks.md` is saved.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-18-MEMORY-todo-pair-test.md`
- `/tmp/compozy-smux-todo-api-20260418-194527/`
- `/tmp/compozy-smux-todo-api-20260418-194527/.compozy/tasks/todo-api/{_prd.md,adrs/adr-001.md}`
- `/tmp/compozy-smux-todo-api-20260418-194527/.tmux-smux.sock`
- Commands: `tmux -S /tmp/compozy-smux-todo-api-20260418-194527/.tmux-smux.sock ...`, `TMUX_BRIDGE_SOCKET=/tmp/compozy-smux-todo-api-20260418-194527/.tmux-smux.sock TMUX_PANE=%0 tmux-bridge ...`, `compozy start --help`, `compozy exec --help`
