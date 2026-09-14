# Approved MCP package mapping

`approved.json` freezes the 17 server definitions from `catalog/mcp.json` at the
pre-feature commit recorded in the fixture, with the SHA-256 of those source bytes.
It is test evidence only; runtime code and publication never read this fixture.

The expected extension server grammar preserves pinned npm/uvx versions, Docker
image digest and arguments, remote URLs, default scope and typed inputs. MCP
`registration: auto` maps to extension `registration: dynamic`; input bindings
name env/secret_env input IDs, and URL-bound inputs add empty placeholders while
preserving fixed query values. These explicit expectations were derived from the
pre-feature source, independently of the package manifests under test.

Do not regenerate expectations from current package output. A future curation
change must update its approved definitions and this fixture together with its
source evidence. The canonical owner is TestManifestCuratedMCPPackages in the
existing extension manifest suite; publisher tests separately preserve existing
extension acquisition metadata and artifact bytes.
