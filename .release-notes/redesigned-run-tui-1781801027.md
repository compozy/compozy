---
title: Redesigned run TUI
type: highlight
---

The interactive terminal UI for `compozy tasks run`, `compozy exec`, and `compozy reviews watch` has been rebuilt from the ground up. The wizard (workspace/agent/model selection and validation) and the live execution view now share a single, consistent layout system with a redesigned sidebar, timeline, and run summary.

### What changed

- **Wizard flow.** Workspace, runtime, and validation steps were reworked for clearer state, tighter spacing, and more legible status. The validation form and run summary render with the same theme as the execution view.
- **Execution layout.** The sidebar, timeline, and summary panes were redesigned for denser, more readable run state — per-job status, attempts, and streamed ACP activity are easier to follow at a glance.
- **Multi-run tabs.** The `--multiple` tab strip was polished so the brand renders once, the spinner survives idle/active tab switches, and an idle tab no longer leaves a dangling spinner.

### Why it matters

This is a visual and structural overhaul of the entire run experience. Existing commands and flags are unchanged — only the presentation and interaction model improved. It also lays the groundwork for the new interactive job control described in the "Steer running agents" note.
