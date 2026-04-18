Goal (incl. success criteria):

- Create a new skill under `.agents/skills/` that orchestrates a tmux/smux-based three-pane workflow: an orchestrator pane, a Codex pane that owns `cy-create-techspec` and `cy-create-tasks`, and a Claude Code pane that acts as the peer reviewer over `tmux-bridge`.
- Success requires a valid skill directory structure, metadata validated by `skill-best-practices`, a lean `SKILL.md`, at least one deterministic helper script, and instructions that end with `compozy start`.

Constraints/Assumptions:

- Follow repository `AGENTS.md` and `CLAUDE.md`; do not touch unrelated files.
- Required skills in play for this session: `skill-best-practices`, `smux`, `cy-create-techspec`, `cy-create-tasks`, and `brainstorming`.
- The user explicitly forbids headless orchestration via `codex exec` or `claude -p`; the new skill must use interactive TUIs plus `tmux-bridge`.
- Verified local CLI facts:
  - `codex` interactive sessions accept `--model`; Codex reasoning must be set through config override (`-c reasoning_effort="xhigh"`).
  - `claude` interactive sessions accept `--model`.
  - `tmux-bridge` is installed and exposes `read`, `message`, `keys`, `name`, `list`, `id`, and `doctor`.
  - `compozy start` exists in the installed CLI and supports `--ide`, `--model`, and `--reasoning-effort`.
- The user corrected the Claude model typo: use `opus`, not `pus`.

Key decisions:

- Name the new skill `smux-compozy-pairing`.
- Keep artifact ownership single-writer: Codex writes the TechSpec and task artifacts; Claude remains the architectural peer unless explicitly reassigned.
- Use a small helper script to emit shell-safe environment variables and exact launch commands for tmux panes and the final `compozy start` invocation.
- Normalize the Claude launch model to `opus` based on the user's correction and the local CLI help.

State:

- In progress after post-verify skill tightening discovered during the live tmux test.

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
- Verified the helper script output against the local repository root.
- Ran fresh repository verification successfully with `make verify`.

Now:

- Keep the repo skill aligned with findings from the disposable todo-api session.

Next:

- Re-run `make verify` after the latest skill edits before closing the repository task.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-18-MEMORY-smux-compozy-pairing.md`
- `.agents/skills/smux-compozy-pairing/`
- `.agents/skills/skill-best-practices/{SKILL.md,assets/SKILL.template.md,references/checklist.md,scripts/validate-metadata.py}`
- `.agents/skills/{smux,cy-create-techspec,cy-create-tasks,brainstorming}/SKILL.md`
- `skills/compozy/references/{cli-reference.md,workflow-guide.md}`
- Commands: `codex --help`, `claude --help`, `tmux-bridge --help`, `compozy start --help`, `python3 .../validate-metadata.py`, `python3 .agents/skills/smux-compozy-pairing/scripts/render-session-plan.py --feature-name daemon --repo-root "$PWD"`, `make verify`
