# Supported harnesses

This file is the single source of truth for harness support in `skill-doctor`. Reference it instead of repeating harness lists in `SKILL.md`.

## Startup gate

| Harness | Collector ID | Local conversation source |
| --- | --- | --- |
| Warp | `warp` | Read-only Warp conversation databases |
| Claude Code | `claude` | Project-history JSONL |
| Codex | `codex` | Rollout JSONL |
| Pi | `pi` | Pi agent JSONL (`~/.pi/agent/sessions`) |
| Grok Build | `grok` | Grok Build chat_history JSONL (`~/.grok/sessions`) |
| ZCode | `zcode` | ZCode model-io rollout (`~/.zcode/cli/rollout`) |

At startup, identify the harness executing the skill from the runtime context. Do not infer it from conversation files found on disk.

If the executing harness is not listed above, or cannot be identified confidently, stop before creating a report directory or reading conversation history. Tell the user:

> skill-doctor currently supports Warp, Claude Code, Codex, Pi, Grok Build, and ZCode. This run appears to be using an unsupported harness, so no conversations were read.

## Collector source selection

- `--harness auto` scans every locally available supported source and is the default.
- `--harness all` also requests every supported source.
- `--harness <collector-id>` restricts collection to one source from the table.
- A report containing one source uses its collector ID in `inventory.json`; a report containing multiple sources uses `mixed`.

Harness-specific source overrides:

- `--claude-home PATH` — nonstandard Claude Code configuration directory.
- `--codex-home PATH` — nonstandard Codex home.
- `--warp-db PATH` — explicit Warp database; repeatable.
- `--warp-data-dir PATH` — nonstandard Warp channel-data directory.
- `--pi-home PATH` — nonstandard Pi agent home (default `~/.pi/agent`).
- `--grok-home PATH` — nonstandard Grok Build home (default `~/.grok`).
- `--zcode-home PATH` — nonstandard ZCode home (default `~/.zcode`).

## Skill locations

Project skills are discovered from:

- `.agents/skills`
- `.claude/skills`
- `.codex/skills`

Global skills are discovered from the corresponding directories under the user's home and configured harness homes when `--include-global-skills` is set: `~/.claude/skills`, `~/.agents/skills`, `~/.codex/skills`, Pi's skill directory under its agent home (default `~/.pi/agent/skills`), Grok Build's `~/.grok/skills`, and ZCode's `~/.zcode/skills`.
