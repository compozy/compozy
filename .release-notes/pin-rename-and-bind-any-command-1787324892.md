---
title: Pin, rename, and bind any command
type: feature
---

The palette learns your workspace. Pins float the commands you always want first, ranking signals push the ones you actually use, aliases give a command your own vocabulary, and any command can take a chord — including a system-wide one on the desktop app. (#441)

- Pins and recents are workspace-scoped and shared across every attached client, so a pin made in the desktop shell shows up in the browser tab. `personalization = false` turns ranking and recents off as one desired state.
- An alias is 1–32 characters with no whitespace, unique in the workspace, and searchable alongside the command's title.
- Bindings, aliases, and pins are validated against the complete effective keymap. A conflict names the command that currently owns the chord or alias and stores nothing; `--overwrite` transfers it as one atomic change.
- The desktop shell registers global hotkeys — `meta+shift+Space` summons CompozyOS with the palette open by default — and reports per-machine truth for each one: **active**, **captured** by another app, **permission required** (with a shortcut into macOS Accessibility settings), or **unsupported**. A browser tab shows the section disabled with the reason _requires desktop shell_ rather than pretending.
- Settings → Palette exposes the agent fallback and personalization; Settings → Layouts → Shortcuts owns the keymap, aliases, and the global section.

```bash
compozy cmd-palette pin palette.view.sessions --workspace acme
compozy cmd-palette alias set session.new new --workspace acme
compozy cmd-palette bind palette.view.tasks meta+shift+KeyY --workspace acme
compozy cmd-palette bind palette.summon.global meta+shift+Space --global
compozy cmd-palette personalization show --workspace acme -o json
```

```toml
[cmd_palette]
personalization = true

[cmd_palette.aliases]
"session.new" = "new"

[window_manager.global_shortcuts]
"palette.summon.global" = "meta+shift+Space"
```
