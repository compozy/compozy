---
title: Extensions contribute commands and views
type: feature
---

An extension can add its own commands and views to the palette from `resources.cmd_palette`, beside its tools. CompozyOS validates the contribution during `extension build`, `extension validate`, install, and development reload, and prefixes every local ID with the extension name — `capture` from the `notes` extension becomes `ext.notes.capture`. (#441)

- The action union is closed: `tool` calls a tool the same extension owns, `view` opens one of its views, `navigate` opens a CompozyOS app, and `url` opens an external link. Extensions cannot declare client operations.
- A declarative view names a **read-only** tool as its source and returns the shared `v1` view payload, which the daemon validates before rendering. A mutating, destructive, interactive, or open-world tool is rejected at validation time, so opening a view never starts an approval flow.
- A programmable view sets `program: true` and is backed by the public `view.provider` surface, with patch streaming for live updates. Start from the template with `compozy extension init notes --template view-provider-ts`.
- A command can ship a `default_shortcut`. If that chord already belongs to something else, the default stays dormant and the conflict is visible in Settings instead of silently stealing the key.
- Destructive extension commands must declare themselves and supply confirmation copy; the same approval gates apply to them as to core commands.

```ts
resources: {
  cmd_palette: {
    commands: [{
      id: "capture",
      title: "Capture note",
      section: "Notes",
      icon: "pencil",
      action: { kind: "tool", tool: "capture_note" },
      default_shortcut: "alt+shift+KeyN",
    }],
    views: [{
      id: "recent",
      title: "Recent notes",
      kind: "list",
      source: { tool: "list_recent" },
    }],
  },
},
```
