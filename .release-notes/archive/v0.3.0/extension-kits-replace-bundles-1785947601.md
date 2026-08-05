---
title: Extension kits replace Bundles
type: breaking
---

The Bundle surface is gone. Extensions are now the single packaging unit, and installing one is inert: it publishes no tools and no resources until you explicitly enable it. Before enabling, you can preview exactly what an extension would publish and inspect what it is publishing right now, and any extension that declares network access must have its network requirement digest confirmed by a person. (#291)

- `compozy extension preview <name>` shows what enabling would publish without changing state; `compozy extension inventory <name>` shows the live published inventory. Agents get the same reads through `compozy__extensions_preview` and `compozy__extensions_inventory`.
- `compozy extension enable|disable|update|install` accept `--confirm-network-requirement <digest>`, so a network-declaring extension cannot start publishing without an explicit confirmation recorded on the install.
- `compozy extension secrets set|bind|list|unset` manages write-only environment bindings that are scoped per workspace and stored as secret references, never as values.
- Marketplace kinds are now exactly `extension`, `mcp`, and `skill`.

Migration notes: the whole `compozy bundle` command group is removed (`catalog`, `preview`, `activate`, `list`, `get`, `deactivate`, `network-settings`), along with the `compozy__bundles_*` native tools, the `compozy__bundles` toolset, the `bundle` marketplace kind, and the Bundle API surfaces. There is no alias — rebuild bundle-shaped setups as extensions and enable them explicitly.
