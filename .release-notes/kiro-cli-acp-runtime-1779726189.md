---
title: Kiro CLI as a supported ACP execution runtime
type: feature
---

Compozy now ships first-class support for the Kiro CLI as an ACP execution runtime, alongside Claude, Codex, Cursor, and Droid. Selecting `--ide kiro` (or persisting it in workspace config) is enough — Compozy locates `kiro-cli`, probes the ACP adapter, and wires the correct bootstrap arguments per session.

### What's new

- New IDE constant `kiro` and default model `anthropic/claude-opus-4-6` (`internal/core/model/constants.go`).
- New registry entry under `internal/core/agent/registry_specs.go` registering the Kiro runtime with:
  - Command: `kiro-cli`
  - Fixed args: `acp`
  - Probe: `kiro-cli acp --help`
  - Setup agent name: `kiro-cli`
  - Docs: <https://kiro.dev/docs/cli/acp>
- Bootstrap forwards `--model <name>` when set and appends `-a` when the run uses `--access-mode full`.

### Usage

```bash
# One-off run
compozy tasks run my-task --ide kiro --model anthropic/claude-opus-4-6

# Per-workspace default
[tasks.run]
ide = "kiro"
model = "anthropic/claude-opus-4-6"
```

### Requirements

Install the Kiro CLI and ensure `kiro-cli acp` is reachable on `PATH`. Compozy's preflight probes the adapter on first run and surfaces a clear `InstallHint` if the binary is missing.

| IDE      | CLI flag     | Default model               | ACP command    |
| -------- | ------------ | --------------------------- | -------------- |
| Kiro CLI | `--ide kiro` | `anthropic/claude-opus-4-6` | `kiro-cli acp` |
