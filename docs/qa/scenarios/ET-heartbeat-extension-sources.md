---
id: ET-heartbeat-extension-sources
area: ET
title: Extension check-in sources respect workspace and profile scope
persona: Dora
entry_points: extension scaffold; heartbeat source catalog; manual instance check
expected: Scoped extension kinds appear in the catalog and poll before triage; invalid parameters and provider failures remain visible source statuses.
qa_status: pending
overlaps: MS-always-on-check-in-sources
---

1. Scaffold `heartbeat-source-go`, register an `issues` kind with a required string
   `repo` parameter, build it, and enable it in a disposable workspace.
2. Read `agent heartbeat sources <agent> -o json`, HTTP/UDS source discovery and
   `compozy__agent_heartbeat_sources` with `{agent, workspace}`. Verify the same
   namespaced kind, schema, script approvals, and limits.
3. Configure `context.sources` with that kind and valid `with` parameters. Run a dry
   check and verify keyed items reach the digest with source provenance.
4. Use invalid parameters. Verify `source_params_invalid` without calling the provider.
5. Read from another workspace and User scope. A workspace-only installation must
   be unavailable. Enable a profile-global instance and verify User scope uses it.
6. Return a typed `rate_limited` error, then crash the fixture process. Verify
   `source_rate_limited` and `source_extension_failed`; restore it and recheck.

Focused automation covers the manager subprocess, scope selection, schema rejection,
SDK stdio, daemon digest, and HTTP/UDS catalog paths. The full public-interface walk
and `TestDaemonE2EHeartbeatSourceCatalog` run in CI/integration QA.

## Bundled GitHub provider

1. Enable `forge-github` and declare `forge-github/issues` with `with.repo` in the
   disposable agent's `HEARTBEAT.md`. Verify the catalog title is "New GitHub issues"
   and its parameter schema lists `repo`, `labels`, `exclude_labels`, and `backfill`.
2. Poll without state: existing open issues produce no items. Open a new issue and
   check again: verify `owner/repo#number`, author, labels, bounded body excerpt,
   URL, and linked open PRs. Closing references have `closes: true`; other PR
   references have `closes: false`; closed PRs are absent.
3. Exceed the batch size. Verify oldest issues arrive first, `more` is true, and
   later checks deliver the remainder. With several pages, verify each poll makes only one issue-page request and resumes without skipping an older issue, including when a page needs several batches. Exclude a label and verify excluded issues
   do not reappear. In a new source, use `backfill: "24h"` and verify its time window.
4. Remove credentials, use an expired token, and simulate a secondary rate limit.
   Verify `source_credential_absent`, `source_credential_expired`, and
   `source_rate_limited` respectively. Restore credentials and recheck without
   losing issues. Without `with.repo`, verify GitHub origin inference and the
   actionable `source_params_invalid` error for a non-GitHub origin.

Provider automation: `TestProviderIssues`, `TestProviderIssuesErrors`, and
`TestProviderIssuesPagination` in `extensions/forge/github/provider_test.go` cover
UT-050–UT-057 through the real HTTP client against an `httptest` GitHub server.
`TestRunProviderForgeProtocolConformanceIT035` also checks source dispatch through
SDK stdio. The live agent check-in journey (declare, Try it now, gate, approval, Web) is
`MS-always-on-check-in-sources`; this scenario keeps the authoring, scope and provider matrix.
