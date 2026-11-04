---
id: ET-heartbeat-extension-sources
area: ET
title: Extension check-in sources respect workspace and profile scope
persona: Dora
entry_points: extension scaffold; heartbeat source catalog; manual instance check
expected: Scoped extension kinds appear in the catalog and poll before triage; invalid parameters and provider failures remain visible source statuses.
qa_status: pending
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
