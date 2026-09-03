# CH-terminal-platform-ladder: Check each platform rung against what it actually promises

```yaml
charter:
  id: CH-terminal-platform-ladder
  mission: "As Dora, run the terminal on each platform rung the product ships and confirm every surface promises exactly what that rung can deliver — a full interactive terminal where it exists, command execution and nothing more where it does not, and no control anywhere that would hang if used."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-terminal-windows
  scenarios: [ET-terminal-windows-parity]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "On a local Windows project: open a terminal, type, read output, resize, detach and reattach, and record; confirm the lifecycle matches what macOS and Linux offer."
      - "On Windows, start a command that spawns child processes, close the terminal, and confirm the whole process tree exits with no orphan and no leaked handle; then run a bounded one-shot command and confirm its output and exit code."
      - "On a Windows sandbox project and on a remote sandbox project, confirm command execution still works while every interactive control and every interactive capability claim is absent — not present and disabled."
      - "Request an interactive terminal on a rung that cannot offer one, through the browser, the command line, and the agent tools, and confirm each refusal names the platform limit and points at the execute-only path."
      - "Compare a macOS or Linux baseline against the matrix below and record any row where the observed behaviour and the promised rung disagree."
    must_avoid:
      - "Do not settle limits, caps, or configuration — CH-terminal-capacity-and-config owns those; do not treat a cross-compiled build as a Windows verdict, the walk needs a real Windows runtime."
```

## Platform matrix expectations

The rungs are honest about themselves; a surface that promises more than its rung is the finding.

| Rung | Interactive terminal | One-shot execution | Attach, resize, control transfer | Recording | Capability reported as |
|---|---|---|---|---|---|
| macOS (local project) | Yes — full, including full-screen programs | Yes | Yes | Yes | interactive |
| Linux (local project) | Yes — full, including full-screen programs | Yes | Yes | Yes | interactive |
| Windows (local project) | Yes — full, through the platform's own console interface | Yes | Yes | Yes | interactive |
| Windows or remote sandbox project | No — refused with the platform named | Yes | No — controls absent, not disabled | No — refused with the platform named | not interactive |

Closing a terminal must end the whole process tree on every rung, and the exit state and retained output
must read the same through the browser and the command line.

## Focus areas

- **ADR-004 (platform ladder)** — three honest rungs; the worst outcome is pretended parity, where a
  terminal works locally and silently hangs elsewhere.
- **Capability honesty** — the capability a surface reports is the capability it can deliver, and an
  unsupported interactive request is refused with the platform named rather than left to fail at use.
- **Process-tree teardown** — closing a terminal releases every child process and every platform handle,
  with no orphan surviving the close.
- **Cross-surface agreement** — the browser, the command line, and the agent tools describe the same rung
  for the same project.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
