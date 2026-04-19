# Runtime Contract

## Role Ownership

- **Orchestrator**
  - Own the tmux session, pane lifecycle, stage routing, checkpoint resolution, and the final `compozy start`.
  - Inspect `.compozy/tasks/<feature-name>/_prd.md` before launch and decide whether to reuse it or route Codex through `cy-create-prd`.
  - Resolve ordinary PRD, TechSpec, and task-breakdown checkpoints inside the session by synthesizing the original user request, repository context, existing artifacts, and Claude's critique.
  - Escalate to the human user only when the source requirements are contradictory or a business-critical decision truly cannot be inferred safely.
  - Re-run `compozy validate-tasks --name <feature-name>` before execution.
- **Codex**
  - Own the final writes under `.compozy/tasks/<feature-name>/`.
  - If the orchestrator selects the PRD path, run `/cy-create-prd <feature-name>` first.
  - Then run `/cy-create-techspec <feature-name>`, followed by `/cy-create-tasks <feature-name>`.
  - Ask Claude for architectural challenges and clarification help over `tmux-bridge`.
  - Surface checkpoints to the orchestrator, not to a human approval queue.
- **Claude Code**
  - Act as the peer reviewer and question partner for Codex.
  - Challenge over-design, missing trade-offs, and weak task boundaries.
  - Help collapse faux-human checkpoints back into orchestrator-resolved decisions whenever the source context is sufficient.
  - Avoid writing the final Compozy artifacts unless the user explicitly reassigns ownership.

## Launch Contract

- Launch Codex interactively:
  - `codex --cd <repo-root> --no-alt-screen --model gpt-5.4 -c reasoning_effort="xhigh" -c developer_instructions="<smux pairing codex overlay>"`
  - Prefer `developer_instructions` for the workflow overlay so Codex keeps its bundled base instructions intact.
  - Only switch to `model_instructions_file` if you intentionally want to replace Codex's full model instructions surface.
- Launch Claude Code interactively:
  - `claude --model opus --permission-mode bypassPermissions --append-system-prompt-file <claude overlay file>`
  - Prefer `--append-system-prompt-file` so Claude keeps its default system prompt and appends the workflow contract.
- Never use headless shortcuts in this workflow:
  - `codex exec`
  - `codex review`
  - `claude -p`
  - `claude --print`
- Launch the final execution through the generated start command or `scripts/run-compozy-start.sh`, not by hand.
  - The wrapper strips inherited interactive session variables before `compozy start`.
  - Current required sanitize set: `CODEX_THREAD_ID`, `TMUX`, and `TMUX_PANE`.

## Messaging Contract

- Use `tmux-bridge message` for every agent-to-agent prompt.
- Respect the `smux` read guard on every interaction.
- Let Codex and Claude talk directly when they are iterating on design questions.
- Route routine checkpoints back through the orchestrator pane for autonomous resolution.
- Do not poll for replies. Wait for the reply to arrive in the pane that initiated the message.
- Use raw `tmux` only for session management, pane creation, and initial TUI launch. Do not use raw `tmux send-keys` for normal agent-to-agent conversation once the workers are live.
- Treat the orchestrator's checkpoint replies as authoritative unless it explicitly says the run is blocked on a human question.

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
- **Worker to orchestrator for checkpoint resolution**
  - `tmux-bridge read <orchestrator-label> 20`
  - `tmux-bridge message <orchestrator-label> '<exact question + recommendation>'`
  - `tmux-bridge read <orchestrator-label> 20`
  - `tmux-bridge keys <orchestrator-label> Enter`

## Stage Gates

1. **PRD gate**
   - The orchestrator checks whether `.compozy/tasks/<feature-name>/_prd.md` already exists.
   - If it does, the workflow uses that PRD.
   - If it does not, Codex runs `/cy-create-prd <feature-name>` unless the original user request explicitly said to skip PRD creation.
   - The orchestrator resolves ordinary PRD checkpoints internally and only escalates on a true blocker.
   - The phase ends when `.compozy/tasks/<feature-name>/_prd.md` exists, or when the original user request explicitly waives PRD creation.
2. **TechSpec gate**
   - The PRD gate must already be resolved.
   - Codex runs `/cy-create-techspec <feature-name>`.
   - The orchestrator resolves ordinary TechSpec checkpoints internally and only escalates on a true blocker.
   - The phase ends only when `.compozy/tasks/<feature-name>/_techspec.md` exists.
3. **Task gate**
   - Codex runs `/cy-create-tasks <feature-name>`.
   - The orchestrator resolves ordinary task-breakdown checkpoints internally and only escalates on a true blocker.
   - The phase ends only when `_tasks.md` and `task_*.md` files exist and `compozy validate-tasks --name <feature-name>` exits `0`.
4. **Execution gate**
   - The orchestrator runs the generated start wrapper for `compozy start`.
   - Effective command shape:
     - `env -u CODEX_THREAD_ID -u TMUX -u TMUX_PANE compozy start --name <feature-name> --ide codex --model gpt-5.4 --reasoning-effort xhigh`
   - The run uses the generated task set without switching runtimes mid-flight.
