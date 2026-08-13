# Hosted extension tool bootstrap QA

- **Issue:** [compozy/compozy#371](https://github.com/compozy/compozy/issues/371)
- **Branch:** `fix/hosted-extension-tool-bootstrap`
- **Scenario:** `ET-048`
- **Result:** PASS
- **Lab manifest:** `/home/franciscpd/dev/qa-labs/compozy-hosted-extension-tool-bootstrap-20260813-151539-252959-lab/qa-artifacts/qa/bootstrap-manifest.json`

## Result

The daemon now includes extension-host tools in the initial hosted MCP projection. A workspace
extension created from the official `tool-provider-go` template was active before a new managed
session bound its hosted MCP server. The live Codex session resolved `ext__qa_hosted__search` as
registered, available, authorized, executable, and callable, then called it with `query=alpha` and
received `No results for alpha`.

This proof is extension-generic: the fixture and live lab use `hello` and `qa-hosted`, respectively.
Neither the implementation nor the tests refer to Batuta or dev-cycle. Vendor MCP providers keep
their existing deferred discovery behavior.

## Verification

- `mise exec -- go test -race ./internal/daemon -count=1` — PASS
- `mise exec -- go test -race -tags=integration ./internal/daemon -run TestDaemonE2EExtensionDistributionAcrossIsolatedHomes -count=1 -v` — PASS
- `mise exec -- golangci-lint run ./internal/daemon/...` — PASS, zero issues
- `mise exec -- make build` — PASS
- Strict QA audit — PASS, no blockers or warnings
- Teardown — `clean: true`, no survivors

Live evidence:

- Session: `sess-67821081fd92dcde`
- Tool discovery: sequences 12–13
- Tool call/result: sequences 15–16
- Result: `No results for alpha`
- Evidence: `/home/franciscpd/dev/qa-labs/compozy-hosted-extension-tool-bootstrap-20260813-151539-252959-lab/qa-artifacts/qa/hosted-extension-proof.json`

## Compozy Impact Audit

- **Native tools:** no `compozy__*` or extension ToolID, descriptor, schema, digest, risk flag, or
  capability gate changed. The hosted MCP bootstrap now includes the already-callable
  `ext__<extension>__<tool>` descriptors. Unit and live hosted-MCP tests cover discovery and call.
- **Extensibility and hooks:** extension-host providers now participate in initial hosted MCP
  discovery; external vendor MCP providers remain deferred. Extension manifests, capabilities,
  subprocess protocol, hooks, resources, bridge SDKs, and config lifecycle are unchanged.
- **Workspace data isolation:** extension discovery and calls keep the bound session workspace scope.
  The E2E installs the extension in an isolated consumer home and invokes it only from its registered
  workspace; no new global, workspace, session, or agent datum is introduced.
- **Official Compozy skill:** no update required. Existing extension authoring and tool-discovery
  guidance already describes extension ToolIDs and live descriptor resolution; this fix restores the
  documented hosted MCP projection behavior without adding a command, schema, or operator workflow.
