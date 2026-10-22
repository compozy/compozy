# Issue 895: session-bound profile agent resolution

## Scope and charter

Ada uses the CLI and public API to find extension agents in the selected profile,
create a session, and inspect its owner. Targeted tour: configuration and scope.
Canary: the default profile must not expose a profile-only extension agent.
The broader Web and skill-precedence journeys retain their previous evidence.

## Session matrix

| Journey | Persona | Surface | Status |
| --- | --- | --- | --- |
| Profile agent catalogs and session ownership | Ada | CLI, HTTP/UDS | Pass |

## Automated integration evidence

`CGO_ENABLED=1 go test -race -tags integration ./internal/daemon -run '^TestSubagentProfileCatalogIntegration$' -count=1` passed.
A real isolated daemon, SQLite, and ACP subprocess exercised profile-default creation,
capability discovery, successful child completion, foreign-profile rejection, and
native status/list owner labels. ACP was a fixture provider, not a live LLM.
Focused native catalog and CLI command regressions also passed with `-race`.

## Session debrief

The compiled CLI found bundled open-design agents through `session_identity` without
`COMPOZY_PROFILE`; repeated reads agreed. The default catalog excluded the designer.
Creating without an explicit agent selected `open-design-designer`; explicitly choosing
`general` preserved that choice. HTTP agent and session reads independently confirmed
the same profile and ownership. No live LLM call was needed for this catalog journey.

The fresh lab initially lacked a default provider, so session creation returned the
expected actionable configuration error. Configuring `defaults.provider=codex` in
that isolated home completed the prerequisite. No host runtime was changed.

Evidence: isolated lab `compozy-issue-895-profile-catalog-20261022-212248-376533-lab`,
`qa-artifacts/qa/journey-log.jsonl` and the referenced JSON responses. Lab teardown
confirmed `clean: true` with no survivors. The strict QA evidence audit passed. PR CI/reviewer
verification remains pending.

The first gate run exposed two existing OpenCode launch-origin assertions comparing
the macOS `/var` temporary-directory alias with canonical `/private/var` paths.
The unchanged suite and full `make gate` passed with `TMPDIR=/private/tmp`: zero
lint issues in normal/integration configurations and all affected Go suites passed
with the race detector. Gate records and the strict audit report are retained with
the lab evidence.
