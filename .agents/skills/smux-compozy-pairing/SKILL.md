---
name: smux-compozy-pairing
description: Orchestrates an interactive tmux-based pairing workflow where Codex authors a Compozy TechSpec, Claude Code challenges assumptions over tmux-bridge, and the orchestrator advances the run through cy-create-tasks and compozy start. Use when a feature already has enough context for collaborative TUI-driven spec and task generation. Don't use for headless automation, single-agent drafting, or flows that call codex exec or claude -p.
---

# Smux Compozy Pairing

## Procedures

**Step 1: Validate inputs and build the launch plan**
1. Require a feature name that resolves to `.compozy/tasks/<feature-name>/`.
2. Execute `eval "$(python3 scripts/render-session-plan.py --feature-name "<feature-name>" --repo-root "$PWD")"` to load the emitted shell assignments into the current shell.
3. Confirm that `tmux`, `tmux-bridge`, `codex`, `claude`, and `compozy` are available in `PATH`.
4. Read `references/runtime-contract.md` before launching the worker TUIs.

**Step 2: Bootstrap the tmux workspace**
1. If `$TMUX` is empty, execute `tmux new-session -s "$SESSION_NAME" -c "$REPO_ROOT"` and continue inside that attached session before using `tmux-bridge`.
2. Rename the active window with `tmux rename-window "$WINDOW_NAME"`.
3. Run `tmux-bridge doctor` after entering the tmux session and before sending the first message.
4. Label the current pane with `tmux-bridge name "$(tmux-bridge id)" "$ORCHESTRATOR_LABEL"`.
5. Create two worker panes rooted at `"$REPO_ROOT"` and rebalance the layout:
   - `CODEX_PANE="$(tmux split-window -hPF '#{pane_id}' -c "$REPO_ROOT")"`
   - `CLAUDE_PANE="$(tmux split-window -vPF '#{pane_id}' -c "$REPO_ROOT")"`
   - `tmux select-layout tiled`
6. Label the worker panes with `tmux-bridge name "$CODEX_PANE" "$CODEX_LABEL"` and `tmux-bridge name "$CLAUDE_PANE" "$CLAUDE_LABEL"`.
7. Launch the interactive workers with raw tmux pane control:
   - `tmux send-keys -t "$CODEX_PANE" -l -- "$CODEX_LAUNCH"`
   - `tmux send-keys -t "$CODEX_PANE" Enter`
   - `tmux send-keys -t "$CLAUDE_PANE" -l -- "$CLAUDE_LAUNCH"`
   - `tmux send-keys -t "$CLAUDE_PANE" Enter`
8. Never launch `codex exec`, `codex review`, `claude -p`, or `claude --print` in this workflow.
9. Use raw `tmux` only for pane lifecycle and TUI bootstrap. Once the workers are running, route every orchestrator-to-worker and worker-to-worker exchange through `tmux-bridge`.

**Step 3: Brief the workers**
1. Read `assets/boot-prompts.md`.
2. Send the Codex boot prompt to `"$CODEX_LABEL"` with `tmux-bridge message`.
3. Send the Claude boot prompt to `"$CLAUDE_LABEL"` with `tmux-bridge message`.
4. Respect the `smux` read guard on every interaction: read, message or type, read again, then press Enter.
5. Ensure the boot prompts include literal `tmux-bridge read`, `tmux-bridge message`, and `tmux-bridge keys` command sequences for Codex-to-Claude, Claude-to-Codex, and worker-to-orchestrator replies.
6. Keep Codex as the sole writer of `_techspec.md`, ADRs, `_tasks.md`, and `task_*.md` unless the user explicitly reassigns ownership.

**Step 4: Resolve the PRD gate**
1. Ask the human user whether the workflow should use an existing PRD or create one first with `"$PRD_COMMAND"`.
2. If the user chooses PRD creation, instruct Codex to run `"$PRD_COMMAND"` before any TechSpec work.
3. Require Codex to consult Claude directly over `tmux-bridge` for requirement pressure, scope control, and user-question rehearsal during the PRD phase.
4. Only surface human checkpoints for decisions that are still genuinely open. If the user already locked a choice during the same run, carry it forward instead of re-litigating it as a fresh menu.
5. When `cy-create-prd` reaches a required human checkpoint, have Codex message `"$ORCHESTRATOR_LABEL"` with the exact question plus its current recommendation.
6. If the choice is already strongly constrained by accepted PRD answers, existing ADRs, or explicit non-goals, prefer a single-option confirmation instead of an A/B/C/D vote.
7. Relay that checkpoint to the human user and pause until the answer arrives.
8. Forward the human answer back to Codex and let the PRD flow continue.
9. Do not advance until either:
   - `"$PRD_PATH"` exists and Codex confirms the PRD was saved after explicit user approval, or
   - the human user explicitly declines PRD creation and instructs the workflow to continue directly to TechSpec.

**Step 5: Run the TechSpec phase**
1. Instruct Codex to run `"$TECHSPEC_COMMAND"` only after the PRD gate is resolved.
2. Require Codex to consult Claude directly over `tmux-bridge` for design challenges, trade-off checks, and question rehearsal.
3. Do not reopen technical choices that are already effectively closed by the approved PRD, accepted ADRs, or explicit non-goals as a fresh multi-option vote.
4. For those already-constrained choices, have Codex either carry the decision forward directly or ask the human for a single-option confirmation if a checkpoint is still useful.
5. When `cy-create-techspec` reaches a required human checkpoint, have Codex message `"$ORCHESTRATOR_LABEL"` with the exact question plus its current recommendation.
6. Relay that checkpoint to the human user and pause until the answer arrives.
7. Forward the human answer back to Codex and let the TechSpec flow continue.
8. Do not advance until `"$TECHSPEC_PATH"` exists and Codex confirms the TechSpec was saved after explicit user approval.

**Step 6: Run the task phase**
1. Instruct Codex to run `"$TASKS_COMMAND"` only after the TechSpec is approved and saved.
2. Apply the same human-checkpoint loop for the task-breakdown review required by `cy-create-tasks`.
3. Re-run `"$VALIDATE_COMMAND"` after Codex reports success, even if `cy-create-tasks` already validated the task set internally.
4. Do not advance until validation exits `0` and `"$TASKS_DIR"` contains `_tasks.md` plus at least one `task_*.md`.

**Step 7: Execute the workflow**
1. Run `"$START_COMMAND"` from the orchestrator pane after task validation passes.
2. Keep the execution runtime aligned with Codex: `--ide codex --model gpt-5.4 --reasoning-effort xhigh`.
3. Let the run finish through the normal Compozy cockpit or the output mode already encoded in the command.
4. Report the resulting `compozy start` state back to the human user.

## Error Handling
* If `tmux-bridge doctor` fails, fix tmux connectivity before creating panes or sending messages.
* If a worker pane exits or never reaches its TUI prompt, relaunch only that pane and resend its boot prompt.
* If Claude cannot run `tmux-bridge` because Bash is blocked by its permission mode, relaunch the Claude pane with `claude --model opus --permission-mode bypassPermissions`, then resend the Claude boot prompt.
* If Codex or Claude receives a `[tmux-bridge from:...]` message, reply to the pane ID from the header instead of answering locally.
* If Codex attempts to shortcut the workflow with `codex exec`, `claude -p`, or another headless command, stop, relaunch the worker interactively, and restart the current phase.
* If either worker discusses the design locally without using `tmux-bridge` for peer communication, resend the boot prompt with the explicit command snippets from `assets/boot-prompts.md` and restart that exchange.
* If the PRD gate reveals that no PRD exists and the human user wants one, never hand-write `_prd.md`. Route the work through `"$PRD_COMMAND"` instead.
* If Codex reopens a choice that is already constrained by accepted PRD answers, existing ADRs, or explicit non-goals as a fresh A/B/C/D menu, stop that checkpoint and restate it as either carry-forward context or a single-option confirmation.
* If the human user rejects the TechSpec or task breakdown, send that rejection back to Codex, keep the panes alive, and continue the same phase instead of rebuilding the session.
