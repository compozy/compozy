# Curated Marketplace Catalog

`sources.json` owns listing metadata, the artifact base URL and the publication timestamp. `packages/<entry_id>/` owns each extension's manifest and packaged files;
`marketplaces.json` owns the ordered plugin marketplace presets. Version, inputs, archive bytes and
SHA-256 digests come from the packages, not from hand-edited feeds.

Generate and validate the v3 feed family from the repository root:

```bash
go run ./cmd/compozy-catalog publish ./catalog ./catalog
go run ./cmd/compozy-catalog validate ./catalog
```

For an isolated publication, replace the second `./catalog` with another output directory. The
publisher stages the complete output, validates its manifests and artifacts through the production
installer and checks packaged inputs against v3 entries before replacing output files. Artifacts are written before feeds. Invalid sources leave
the existing output untouched; filesystem failures during replacement are reported and require a
retry. Unrelated files in the destination are preserved.

Generated output:

- `v3/extensions.json`: the nineteen extensions, including seventeen packaged MCP servers.
- `v3/marketplaces.json`: ordered plugin marketplace presets.
- `artifacts/*.tar.gz`: deterministic package archives referenced by the extension feed.

Do not edit generated feeds or artifacts. Only the v3 family is published; root MCP/skill
feeds, semantic v2 adapters and the standalone Documentation Writer listing are retired.
The two listed pre-existing extension packages keep their authored identities, acquisition references,
versions and artifact bytes. The 17 MCP packages are validated against fixed pre-feature
curation evidence under `internal/extension/testdata/mcp_to_extension/`.

Repository Orientation is no longer listed in the official feed. Its previously published package
and artifact remain available for existing references; removing the listing does not uninstall it.

The daemon reads `<base_url>/v3/extensions.json` and `<base_url>/v3/marketplaces.json`.
Missing or invalid v3 documents report a source failure without reading root feeds.
Mirrors keep their configured base URL and must serve the v3 family.

## Packaged MCP manifests

Each package declares its launch in `resources.mcp_servers`, its default scope, and any OAuth policy.
Remote packages without an upstream package version use extension version `1.0.0`; this versions the
manifest, not the hosted service. Manifest OAuth uses `method = "oauth"` and
`registration = "dynamic"` for automatic registration. An optional `issuer_url` restricts discovery
to that issuer. `default_scope` is the installation default; it does not broaden an installed
extension's workspace scope.

Use `[[inputs]]` for install values. Every declaration needs a unique id and binding, a prompt, and a
type (`string`, `identifier`, `boolean`, or `secret`). An environment binding must name a variable in
at least one server's `env` or `secret_env` map, with the input id as its value. A URL binding must name
a query parameter already declared by an HTTP server. Secrets require `secret_env`, cannot have
defaults, and cannot bind URL parameters. Boolean defaults are TOML booleans; other non-secret defaults
are strings. Required environment inputs contribute their variable names to `requires_env`.

Validate a manifest without starting its server:

```bash
go run ./cmd/compozy extension validate ./catalog/packages/context7 --output json
```

Listing icons accept HTTPS PNG/SVG/WebP URLs or matching data URLs up to 64 KiB. Publication rejects
invalid icons; runtime decoding drops an invalid optional icon and returns an entry diagnostic.
