# Runtime Contract

## Role Ownership

- **Orchestrator**
  - Own the tmux session, pane lifecycle, the initial PRD decision, human-facing checkpoints, and the final `compozy start`.
  - Surface the exact review and approval questions required by `cy-create-techspec` and `cy-create-tasks`.
  - Surface the initial question of whether the workflow should use an existing PRD or create one with `cy-create-prd`.
  - Re-run `compozy validate-tasks --name <feature-name>` before execution.
- **Codex**
  - Own the final writes under `.compozy/tasks/<feature-name>/`.
  - If the orchestrator selects the PRD path, run `/cy-create-prd <feature-name>` first.
  - Then run `/cy-create-techspec <feature-name>`, followed by `/cy-create-tasks <feature-name>`.
  - Ask Claude for architectural challenges and clarification help over `tmux-bridge`.
- **Claude Code**
  - Act as the peer reviewer and question partner for Codex.
  - Challenge over-design, missing trade-offs, and weak task boundaries.
  - Avoid writing the final Compozy artifacts unless the user explicitly reassigns ownership.

## Launch Contract

- Launch Codex interactively:
  - `codex --cd <repo-root> --no-alt-screen --model gpt-5.4 -c reasoning_effort="xhigh"`
- Launch Claude Code interactively:
  - `claude --model opus --permission-mode bypassPermissions`
- Never use headless shortcuts in this workflow:
  - `codex exec`
  - `codex review`
  - `claude -p`
  - `claude --print`

## Messaging Contract

- Use `tmux-bridge message` for every agent-to-agent prompt.
- Respect the `smux` read guard on every interaction.
- Let Codex and Claude talk directly when they are iterating on design questions.
- Route only human-approval checkpoints back through the orchestrator pane.
- Do not poll for replies. Wait for the reply to arrive in the pane that initiated the message.
- Use raw `tmux` only for session management, pane creation, and initial TUI launch. Do not use raw `tmux send-keys` for normal agent-to-agent conversation once the workers are live.

## Required Worker Command Patterns

- **Codex to Claude**
  - `tmux-bridge read <claude-label> 20`
  - `tmux-bridge message <claude-label> '<question or review request>'`
  - `tmux-bridge read <claude-label> 20`
  - `tmux-bridge keys <claude-label> Enter`
- **Claude reply to Codex**
  - Extract the sender pane id from the `[tmux-bridge from:...]` header.
  - `tmux-bridge read <sender-pane-id> 20`
  - `tmux-bridge message <sender-pane-id> '<answer or critique>'`
  - `tmux-bridge read <sender-pane-id> 20`
  - `tmux-bridge keys <sender-pane-id> Enter`
- **Worker to orchestrator for human checkpoints**
  - `tmux-bridge read <orchestrator-label> 20`
  - `tmux-bridge message <orchestrator-label> '<exact question + recommendation>'`
  - `tmux-bridge read <orchestrator-label> 20`
  - `tmux-bridge keys <orchestrator-label> Enter`

## Stage Gates

1. **PRD gate**
   - The orchestrator asks whether an existing PRD should be used or whether Codex should create one with `/cy-create-prd <feature-name>`.
   - If the user selects PRD creation, Codex runs `/cy-create-prd <feature-name>`.
   - The human user must approve the PRD before the phase can close.
   - The phase ends when `.compozy/tasks/<feature-name>/_prd.md` exists, or when the user explicitly declines PRD creation.
2. **TechSpec gate**
   - The PRD gate must already be resolved.
   - Codex runs `/cy-create-techspec <feature-name>`.
   - The human user must approve the TechSpec before the phase can close.
   - The phase ends only when `.compozy/tasks/<feature-name>/_techspec.md` exists.
3. **Task gate**
   - Codex runs `/cy-create-tasks <feature-name>`.
   - The human user must approve the task breakdown before the phase can close.
   - The phase ends only when `_tasks.md` and `task_*.md` files exist and `compozy validate-tasks --name <feature-name>` exits `0`.
4. **Execution gate**
   - The orchestrator runs `compozy start --name <feature-name> --ide codex --model gpt-5.4 --reasoning-effort xhigh`.
   - The run uses the generated task set without switching runtimes mid-flight.
