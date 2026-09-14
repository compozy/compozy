# Catalog brand icons

These files identify the services provided by the packaged MCP extensions. Source URLs and SHA-256 digests are recorded in `sources.json`. Downloads retain the original pixels and geometry. Where an official favicon is an ICO container, the original embedded PNG bytes are extracted without resizing or recoloring.

GitHub and Linear reuse the existing `@compozy/ui` components, rendered as standalone SVGs for the feed image field. No app-local brand component is introduced. Other assets come from each vendor website or official repository.

The complete icon inventory, source pages, asset paths, SHA-256 digests, byte counts, and conversions are maintained in [sources.json](sources.json).

`catalog/sources.json` owns the icon URLs. Republish through `go run ./cmd/compozy-catalog publish ./catalog ./catalog`; only the v3 family is published, and it carries icons.
