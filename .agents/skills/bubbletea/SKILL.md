---
name: bubbletea
description: "Build interactive Go TUIs with Bubbletea, Elm architecture, Lipgloss, layouts, and keyboard/mouse handling. Excludes plain-text CLIs, web/desktop GUIs, and other TUI frameworks."
license: MIT
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# Bubble Tea TUI Development

Work with the repository's installed Bubble Tea/Lip Gloss versions and existing model/update/view structure. Use current official docs for version-specific API changes; do not assume bundled examples match the consuming project's major version.

## Implementation

- Reuse nearby components and state ownership. A small widget does not require a new project scaffold, YAML configuration, hot reload, mouse handling, or effects library.
- Before changing layout, inspect the relevant guidance in `references/golden-rules.md`. Measure the actual frame/padding and clamp small-terminal dimensions; subtracting a fixed two cells is valid only for that exact border.
- Use terminal-cell/ANSI-aware measurement and truncation. Do not slice strings by byte length or assume code points equal display width. Choose clipping, wrapping, or scrolling for the content's behavior; not all bordered text must truncate.
- Keep hit-testing consistent with the rendered layout and focus state. Proportional sizing is useful for flexible panes; fixed sizes are valid for intentional controls.
- Keep effectful work outside rendering and preserve the model/update lifecycle. Validate the changed interaction and relevant narrow/wide terminal sizes; add an owning test only when behavior lacks coverage.

## References

| Concern | Read as needed |
| --- | --- |
| Border sizing, panel alignment | `references/golden-rules.md` |
| Existing widget patterns | `references/components.md` |
| Rendering failure | `references/troubleshooting.md` |
| Unicode/emoji alignment | `references/emoji-width-fix.md` |

Bundled templates are optional starting points for a new application. Adapt their API versions and layout assumptions; do not replace a working project structure or add dependencies it does not need. Install required dependencies through the repository's package workflow, not by hand-editing go.mod. Product behavior, terminal constraints, and project conventions take precedence over illustrative examples.
