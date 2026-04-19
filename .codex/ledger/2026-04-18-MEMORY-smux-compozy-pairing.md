Goal (incl. success criteria):

- Create a new skill under `.agents/skills/` that orchestrates a tmux/smux-based three-pane workflow: an orchestrator pane, a Codex pane that owns `cy-create-techspec` and `cy-create-tasks`, and a Claude Code pane that acts as the peer reviewer over `tmux-bridge`.
- Success requires a valid skill directory structure, metadata validated by `skill-best-practices`, a lean `SKILL.md`, at least one deterministic helper script, and instructions that end with `compozy start`.

Constraints/Assumptions:

- Follow repository `AGENTS.md` and `CLAUDE.md`; do not touch unrelated files.
- Required skills in play for this session: `skill-best-practices`, `smux`, `cy-create-techspec`, `cy-create-tasks`, and `brainstorming`.
- The user explicitly forbids headless orchestration via `codex exec` or `claude -p`; the new skill must use interactive TUIs plus `tmux-bridge`.
- Verified local CLI facts:
  - `codex` interactive sessions accept `--model`; Codex reasoning must be set through config override (`-c reasoning_effort="xhigh"`).
  - `codex` accepts `-c developer_instructions=...`; official OpenAI docs confirm `model_instructions_file`, but using it would replace the bundled base instructions surface.
  - `claude` interactive sessions accept `--model`.
  - `claude` accepts `--system-prompt`, `--append-system-prompt`, `--system-prompt-file`, and `--append-system-prompt-file` in the installed `2.1.111` build.
  - `tmux-bridge` is installed and exposes `read`, `message`, `keys`, `name`, `list`, `id`, and `doctor`.
  - `compozy start` exists in the installed CLI and supports `--ide`, `--model`, and `--reasoning-effort`.
- The user corrected the Claude model typo: use `opus`, not `pus`.

Key decisions:

- Name the new skill `smux-compozy-pairing`.
- Keep artifact ownership single-writer: Codex writes the TechSpec and task artifacts; Claude remains the architectural peer unless explicitly reassigned.
- Use a small helper script to emit shell-safe environment variables and exact launch commands for tmux panes and the final `compozy start` invocation.
- Normalize the Claude launch model to `opus` based on the user's correction and the local CLI help.
- Remove the human approval loop from the skill. The orchestrator must resolve routine checkpoints inside the session and only escalate to the user on a genuine blocker.
- Inject stable worker behavior at launch time instead of relying only on boot prompts:
  - Codex uses `-c developer_instructions=...` so the bundled Codex base instructions remain intact.
  - Claude uses `--append-system-prompt-file ...` so the default Claude system prompt remains intact.

State:

- In progress. The skill contract and wrappers are patched, and fresh repository verification passed after the latest start-command sanitization changes.

Done:

- Read the repository instructions, required skill docs, and related memory ledgers.
- Verified metadata candidate `smux-compozy-pairing` with `skill-best-practices/scripts/validate-metadata.py`.
- Verified local CLI surfaces for `codex`, `claude`, `tmux-bridge`, and installed `compozy`.
- Confirmed the installed `compozy` binary still exposes `start` even though the local `go run ./cmd/compozy` help in this branch differs.
- Created the new skill structure and files:
  - `.agents/skills/smux-compozy-pairing/SKILL.md`
  - `.agents/skills/smux-compozy-pairing/references/runtime-contract.md`
  - `.agents/skills/smux-compozy-pairing/assets/boot-prompts.md`
  - `.agents/skills/smux-compozy-pairing/scripts/render-session-plan.py`
- Tightened the main skill flow so the launch plan is loaded through `eval`, the tmux bootstrap uses an explicit `tmux new-session`, and `tmux-bridge doctor` runs only after entering the tmux session.
- Tightened the skill after the live todo-api test so locked PRD/ADR choices must be carried forward or confirmed as a single option instead of being reopened as fresh A/B/C/D menus during TechSpec.
- Reworked the skill contract so the orchestrator is autonomous instead of relaying routine PRD/TechSpec/task checkpoints to a human.
- Added persistent worker prompt assets:
  - `.agents/skills/smux-compozy-pairing/assets/codex-developer-instructions.md`
  - `.agents/skills/smux-compozy-pairing/assets/claude-append-system-prompt.md`
- Updated `scripts/render-session-plan.py` so:
  - Codex launch includes `-c developer_instructions=...`
  - Claude launch includes `--append-system-prompt-file ...`
- Added shell wrappers so users do not have to hand-assemble launch and phase commands:
  - `.agents/skills/smux-compozy-pairing/scripts/print-session-command.sh`
  - `.agents/skills/smux-compozy-pairing/scripts/run-codex-worker.sh`
  - `.agents/skills/smux-compozy-pairing/scripts/run-claude-worker.sh`
- Added `.agents/skills/smux-compozy-pairing/scripts/run-compozy-start.sh` so the final execution command also comes from the same generated contract.
- Patched `scripts/render-session-plan.py` so `START_COMMAND` strips inherited interactive session variables before launching Compozy:
  - `CODEX_THREAD_ID`
  - `TMUX`
  - `TMUX_PANE`
- Verified the new shell wrappers with `sh -n` and confirmed `print-session-command.sh` emits the exact Codex and `compozy start` commands for a sample feature.
- Verified the current local CLIs support the chosen argument surfaces:
  - `claude --system-prompt-file <file> --version` exits `0`
  - `claude --append-system-prompt-file <file> --version` exits `0`
  - `codex debug prompt-input -c developer_instructions=...` injects the developer overlay into the model-visible input
- Verified the helper script output against the local repository root.
- Ran `make verify` after the latest skill updates; it passed with `DONE 2385 tests, 1 skipped` and `All verification checks passed`.

Now:

- Continue the live end-to-end validation in a disposable tmux workspace using the patched start wrapper.

Next:

- Observe the fresh tmux run through PRD, TechSpec, tasks, and the final `compozy start` execution.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-18-MEMORY-smux-compozy-pairing.md`
- `.agents/skills/smux-compozy-pairing/`
- `.agents/skills/smux-compozy-pairing/assets/{codex-developer-instructions.md,claude-append-system-prompt.md}`
- `.agents/skills/smux-compozy-pairing/scripts/{render-session-plan.py,print-session-command.sh,run-codex-worker.sh,run-claude-worker.sh,run-compozy-start.sh}`
- `.agents/skills/skill-best-practices/{SKILL.md,assets/SKILL.template.md,references/checklist.md,scripts/validate-metadata.py}`
- `.agents/skills/{smux,cy-create-techspec,cy-create-tasks,brainstorming}/SKILL.md`
- `skills/compozy/references/{cli-reference.md,workflow-guide.md}`
- Commands: `codex --help`, `codex debug prompt-input ...`, `claude --help`, `claude --system-prompt-file ... --version`, `claude --append-system-prompt-file ... --version`, `tmux-bridge --help`, `compozy start --help`, `python3 .../validate-metadata.py`, `python3 .agents/skills/smux-compozy-pairing/scripts/render-session-plan.py --feature-name daemon --repo-root "$PWD"`, `sh -n .../run-compozy-start.sh`, `make verify`
