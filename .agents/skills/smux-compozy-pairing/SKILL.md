---
name: smux-compozy-pairing
description: Orchestrates an interactive tmux-based pairing workflow where the current agent acts as an autonomous orchestrator, Codex authors the Compozy PRD/TechSpec/tasks, Claude Code challenges assumptions over tmux-bridge, and the orchestrator advances the run through compozy start without a human approval loop. Use when a feature needs collaborative TUI-driven spec and task generation. Don't use for headless automation, single-agent drafting, or flows that call codex exec or claude -p.
---

# Smux Compozy Pairing

## Procedures

**Step 1: Validate inputs and build the launch plan**
1. Require a feature name that resolves to `.compozy/tasks/<feature-name>/`.
2. Execute `eval "$(python3 .agents/skills/smux-compozy-pairing/scripts/render-session-plan.py --feature-name "<feature-name>" --repo-root "$PWD")"` to load the emitted shell assignments into the current shell.
3. Confirm that `tmux`, `tmux-bridge`, `codex`, `claude`, and `compozy` are available in `PATH`.
4. Read `references/runtime-contract.md` before launching the worker TUIs.
5. Treat `"$CODEX_LAUNCH"` and `"$CLAUDE_LAUNCH"` as authoritative. The helper already injects a Codex workflow overlay via `developer_instructions` and a Claude workflow overlay via `--append-system-prompt-file`.
6. Prefer the shell wrappers under `scripts/` when you need a concrete command string:
   - `scripts/print-session-command.sh --feature-name "$FEATURE_NAME" --repo-root "$REPO_ROOT" --kind codex-launch`
   - `scripts/print-session-command.sh --feature-name "$FEATURE_NAME" --repo-root "$REPO_ROOT" --kind claude-launch`
   - `scripts/print-session-command.sh --feature-name "$FEATURE_NAME" --repo-root "$REPO_ROOT" --kind start`
   - `scripts/run-codex-worker.sh --feature-name "$FEATURE_NAME" --repo-root "$REPO_ROOT"`
   - `scripts/run-claude-worker.sh --feature-name "$FEATURE_NAME" --repo-root "$REPO_ROOT"`
   - `scripts/run-compozy-start.sh --feature-name "$FEATURE_NAME" --repo-root "$REPO_ROOT"`

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
7. Resolve the worker launch commands through the shell wrapper so you cannot accidentally drop a required flag:
   - `CODEX_CMD="$("$SKILL_ROOT/scripts/print-session-command.sh" --feature-name "$FEATURE_NAME" --repo-root "$REPO_ROOT" --kind codex-launch)"`
   - `CLAUDE_CMD="$("$SKILL_ROOT/scripts/print-session-command.sh" --feature-name "$FEATURE_NAME" --repo-root "$REPO_ROOT" --kind claude-launch)"`
8. Launch the interactive workers with raw tmux pane control:
   - `tmux send-keys -t "$CODEX_PANE" -l -- "$CODEX_CMD"`
   - `tmux send-keys -t "$CODEX_PANE" Enter`
   - `tmux send-keys -t "$CLAUDE_PANE" -l -- "$CLAUDE_CMD"`
   - `tmux send-keys -t "$CLAUDE_PANE" Enter`
9. Never launch `codex exec`, `codex review`, `claude -p`, or `claude --print` in this workflow.
10. Use raw `tmux` only for pane lifecycle and TUI bootstrap. Once the workers are running, route every orchestrator-to-worker and worker-to-worker exchange through `tmux-bridge`.

**Step 3: Brief the workers**
1. Read `assets/boot-prompts.md`.
2. Send the Codex boot prompt to `"$CODEX_LABEL"` with `tmux-bridge message`.
3. Send the Claude boot prompt to `"$CLAUDE_LABEL"` with `tmux-bridge message`.
4. Respect the `smux` read guard on every interaction: read, message or type, read again, then press Enter.
5. Ensure the boot prompts include literal `tmux-bridge read`, `tmux-bridge message`, and `tmux-bridge keys` command sequences for Codex-to-Claude, Claude-to-Codex, and worker-to-orchestrator replies.
6. Keep Codex as the sole writer of `_techspec.md`, ADRs, `_tasks.md`, and `task_*.md` unless the user explicitly reassigns ownership.
7. Treat the orchestrator as an autonomous agent, not as a human relay. Routine PRD, TechSpec, and task-breakdown checkpoints must be resolved inside the session unless the source requirements are truly contradictory or missing.
8. Use the launch-time prompt overlays for stable role behavior and use the boot prompts only for run-specific context such as feature slug, pane labels, and exact phase ordering.

**Step 4: Resolve the PRD gate**
1. Inspect `"$PRD_PATH"` before starting the workflow.
2. If `"$PRD_PATH"` already exists, treat it as the active PRD and move to the TechSpec phase.
3. If `"$PRD_PATH"` does not exist, default to instructing Codex to run `"$PRD_COMMAND"` before any TechSpec work unless the original user request explicitly said to skip PRD creation.
4. Require Codex to consult Claude directly over `tmux-bridge` for requirement pressure, scope control, and checkpoint rehearsal during the PRD phase.
5. Only escalate beyond the session if the source requirements are contradictory or a business-critical choice truly cannot be inferred from the prompt, repository context, existing artifacts, or the pair's recommendations.
6. When `cy-create-prd` reaches a required checkpoint, have Codex message `"$ORCHESTRATOR_LABEL"` with the exact question plus its current recommendation.
7. Resolve that checkpoint inside the session. The orchestrator should synthesize the original user goal, repository context, existing PRD/ADR state, and Claude's critique, then send the decision straight back to Codex.
8. If the choice is already strongly constrained by accepted PRD answers, existing ADRs, or explicit non-goals, carry it forward directly or answer with a single-option confirmation instead of reopening it as a fresh menu.
9. Do not advance until either:
   - `"$PRD_PATH"` exists and Codex confirms the PRD was saved, or
   - the original user request explicitly told the run to skip PRD creation.

**Step 5: Run the TechSpec phase**
1. Instruct Codex to run `"$TECHSPEC_COMMAND"` only after the PRD gate is resolved.
2. Require Codex to consult Claude directly over `tmux-bridge` for design challenges, trade-off checks, and question rehearsal.
3. Do not reopen technical choices that are already effectively closed by the approved PRD, accepted ADRs, or explicit non-goals as a fresh multi-option vote.
4. For those already-constrained choices, have Codex either carry the decision forward directly or ask the orchestrator for a single-option confirmation if a checkpoint is still useful.
5. When `cy-create-techspec` reaches a required checkpoint, have Codex message `"$ORCHESTRATOR_LABEL"` with the exact question plus its current recommendation.
6. Resolve that checkpoint inside the session. The orchestrator may briefly pressure-test the recommendation with Claude, but it should not stop for human approval in the normal case.
7. Forward the orchestrator's answer back to Codex and let the TechSpec flow continue.
8. Do not advance until `"$TECHSPEC_PATH"` exists and Codex confirms the TechSpec was saved.

**Step 6: Run the task phase**
1. Instruct Codex to run `"$TASKS_COMMAND"` only after the TechSpec is approved and saved.
2. Apply the same orchestrator-owned checkpoint loop for the task-breakdown review required by `cy-create-tasks`.
3. Re-run `"$VALIDATE_COMMAND"` after Codex reports success, even if `cy-create-tasks` already validated the task set internally.
4. Do not advance until validation exits `0` and `"$TASKS_DIR"` contains `_tasks.md` plus at least one `task_*.md`.

**Step 7: Execute the workflow**
1. Run `"$START_COMMAND"` from the orchestrator pane after task validation passes.
2. Keep the execution runtime aligned with Codex: `--ide codex --model gpt-5.4 --reasoning-effort xhigh`.
3. Treat the generated start wrapper as authoritative because it strips interactive Codex/tmux session variables such as `CODEX_THREAD_ID`, `TMUX`, and `TMUX_PANE` before launching `compozy start`.
4. Let the run finish through the normal Compozy cockpit or the output mode already encoded in the command.
5. Report the resulting `compozy start` state back to the human user.

## Error Handling
* If `tmux-bridge doctor` fails, fix tmux connectivity before creating panes or sending messages.
* If a worker pane exits or never reaches its TUI prompt, relaunch only that pane and resend its boot prompt.
* If Claude cannot run `tmux-bridge` because Bash is blocked by its permission mode, relaunch the Claude pane with `claude --model opus --permission-mode bypassPermissions`, then resend the Claude boot prompt.
* If Codex or Claude receives a `[tmux-bridge from:...]` message, reply to the pane ID from the header instead of answering locally.
* If the current orchestrator pane is only a passive shell and cannot resolve checkpoints autonomously, stop and relaunch the workflow from an actual agent-controlled tmux pane before continuing.
* If Codex attempts to shortcut the workflow with `codex exec`, `claude -p`, or another headless command, stop, relaunch the worker interactively, and restart the current phase.
* If either worker discusses the design locally without using `tmux-bridge` for peer communication, resend the boot prompt with the explicit command snippets from `assets/boot-prompts.md` and restart that exchange.
* If the PRD gate reveals that no PRD exists and the original user request did not explicitly waive PRD creation, never hand-write `_prd.md`. Route the work through `"$PRD_COMMAND"` instead.
* If Codex reopens a choice that is already constrained by accepted PRD answers, existing ADRs, or explicit non-goals as a fresh A/B/C/D menu, stop that checkpoint and restate it as either carry-forward context or a single-option confirmation.
* If a checkpoint genuinely cannot be resolved from the prompt, repository context, existing artifacts, and Claude/Codex recommendations, ask the human user one targeted question, keep the panes alive, and continue the same phase after the answer arrives.
