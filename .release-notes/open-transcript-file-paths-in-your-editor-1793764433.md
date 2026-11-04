---
title: Open transcript file paths in your editor from the desktop app
type: feature
---

Agents often name files in their replies, such as `internal/mcp/hosted_proxy.go:42`. In the CompozyOS desktop app, **Cmd+click** (macOS) or **Ctrl+click** (Windows and Linux) a file path in an assistant message to open it at that line in your editor. An **Open in editor** button also appears on hover or focus.

- CompozyOS detects paths in inline code and in links whose whole text is a path, with an optional `:line` or `:line:col`. Plain text isn't detected.
- Relative paths resolve against the session's working directory, then its worktree, then the project root.
- **Choose the editor** in **Settings › General › Editor command**, or run `compozy config set desktop.editor_command 'code -g {path}:{line}:{col}'`. The value is a command template with `{path}`, `{line}` and `{col}`. Left empty, the system text editor opens the file, without line support.
- Files outside the session's worktree or project, and credential folders such as `~/.ssh` and `~/.aws`, ask you to confirm first. The dialog shows the full path, and Cancel is the default.
- The browser and remote clients (LAN, phone, Gateway) show paths as before and can't start anything on the daemon's machine.

`desktop.editor_command` is set per machine; project and profile config can't set it, and agents can't change it. Single-session reads gain a read-only `working_dir` field.
