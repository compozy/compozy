# Agent kind removal — targeted CLI journey

Scope: #1239, removal after v0.5.2. Persona: CLI operator upgrading existing agent definitions.
An isolated targeted CLI/API/runtime lab contains no live provider credentials. The walk does not prompt a model.

| Journey | Status | Evidence |
| --- | --- | --- |
| Create always-on definition, instance and inspect agent JSON | Pass | CLI/API observations below |
| Reject retired flag and show actionable workspace definition diagnostics | Pass | CLI/API observations below |
| Reject retired table and CLI config paths without changing files | Pass | CLI/API observations below |

CI owns broad integration/E2E and generated-drift gates. Focused package tests are separate from this real-daemon journey.

## Session debrief

The CLI operator created `triage` as `always_on`, created its workspace instance and read its definition again.
A separate HTTP catalog read confirmed the definition. The retired `--kind` exited 1 with
`agent_kind_invalid` and the exact rename guidance. Config get/set/unset exited 1 with the replacement path.
A seeded workspace definition was omitted from the ordinary agent list and appeared in the public
`/api/agents/catalog` with the actionable load diagnostic. Its original bytes remained unchanged.
A config overlay containing the removed table failed `config validate` with the replacement table name.

The first diagnostics probe used the ordinary list, which omits failed definitions; the catalog is the
public diagnostics surface. The boot integration separately confirms that an invalid global definition
refuses boot with the same actionable error, without rewriting files.

The targeted lab used the built CLI, real SQLite and daemon, a unique home/socket/port, and installed
provider processes without sending a model prompt. Live model behavior and Web interaction were outside this removal journey.
Teardown reported `TEARDOWN_CLEAN=true` and removed all owned daemon/provider processes.

Focused tests, integration tests, generated artifacts and CI are recorded separately in the PR.

Final QA verdict: PASS for this targeted journey. PR readiness awaits current-head CI and reviewers.
