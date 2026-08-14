# Config Path Error Message Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every managed agent receive an unambiguous provider-visible message when `compozy__config_get` reads an absent path.

**Architecture:** Fix the safe public message at the HTTP/UDS tool-error serialization boundary by allowing the existing selector to inspect deterministic reasons. Keep the hosted MCP formatter generic and verify that it projects the corrected transport payload without an extension-specific branch.

**Tech Stack:** Go 1.26, Gin HTTP/UDS handlers, Compozy tool contracts, Go MCP SDK, daemon integration tests.

## Global Constraints

- Preserve `tool_not_found`, `config_path_not_found`, HTTP 404, ToolIDs, and all schemas.
- Never expose internal error text.
- Do not add an extension-specific or Batuta-specific fallback.
- Use named `Should...` subtests and run changed Go behavior under the race detector.
- Open the English issue before the first commit; issue #394 satisfies this requirement.

---

### Task 1: Reason-aware public tool error

**Files:**
- Modify: `internal/api/core/tools_test.go`
- Modify: `internal/mcp/hosted_proxy_test.go`
- Modify: `internal/api/core/tool_errors.go`

**Interfaces:**
- Consumes: `tools.ToolError{Code, ReasonCodes}` and `core.ToolErrorResponseForError(error, int, bool)`.
- Produces: `safeToolErrorMessage(status int, code tools.ErrorCode, reasons []tools.ReasonCode) string` behavior consumed by HTTP/UDS serialization and the existing hosted MCP proxy.

**Web/Docs Impact:**
- Web: no route, component, hook, or generated client change; the existing HTTP/UDS error envelope keeps its schema and only corrects the safe message for an existing reason.
- Docs: no `packages/site` change; the operator-facing configuration surface is unchanged, while managed-agent guidance is updated in the official skill in Task 2.

- [ ] **Step 1: Write the failing transport tests**

Add a `TestToolErrorResponses/Should describe a missing config path without reporting the tool missing`
case that constructs a `ToolError` with `ErrorCodeNotFound` and
`ReasonConfigPathNotFound`. Assert HTTP 404, unchanged code/reason, message
`config path not found`, and absence of `tool not found`.

Add a hosted proxy case that constructs the serialized `contract.ToolErrorResponse` fixture at the
MCP boundary, wraps it in the existing `hostedToolResponseError`, and expects exactly
`config_path_not_found: config path not found`.

- [ ] **Step 2: Run tests to verify RED**

Run:

```bash
mise exec -- go test ./internal/api/core ./internal/mcp -run 'Test(ToolErrorResponses|HostedProxy)' -count=1
```

Expected: both new assertions fail with `tool not found` in the actual message.

- [ ] **Step 3: Implement the minimal reason-aware selector**

Pass `ToolError.ReasonCodes` into `safeToolErrorMessage`. Before the existing code switch, return
`config path not found` when the reasons contain `ReasonConfigPathNotFound`. Pass `nil` reasons for
code-only fallback paths. Do not change any other message.

- [ ] **Step 4: Run tests to verify GREEN**

Run:

```bash
mise exec -- env CGO_ENABLED=1 go test -race ./internal/api/core ./internal/mcp -run 'Test(ToolErrorResponses|HostedProxy)' -count=1
```

Expected: PASS with no warnings.

### Task 2: Managed extension-session regression

**Files:**
- Modify: `internal/daemon/daemon_extension_agent_fixture_e2e_integration_test.go`
- Modify: `skills/compozy/references/configuration.md`

**Interfaces:**
- Consumes: the real hosted MCP client bound to an extension-published managed agent.
- Produces: an end-to-end assertion that the provider sees the exact reason/message pair for a neutral absent config path.

**Web/Docs Impact:**
- Web: no route, component, or hook change; the existing managed-session flow is exercised through the hosted MCP transport.
- Docs: update `skills/compozy/references/configuration.md`; no `packages/site` change because no operator workflow, configuration key, or schema changes.

- [ ] **Step 1: Strengthen the existing E2E assertion**

Replace the Batuta-named absent path with `loops.inputs.extension-probe.preference`. Extend the
helper assertion so the config call requires the exact text
`config_path_not_found: config path not found` and rejects `tool not found`; keep the existing
reason-only behavior for unrelated workspace-fence assertions.

- [ ] **Step 2: Verify the E2E would fail without Task 1**

Use the pre-fix commit or the recorded Task 1 RED output to establish that the real hosted result is
`config_path_not_found: tool not found`. Do not revert production code only to manufacture a second
RED run.

- [ ] **Step 3: Run the real hosted MCP E2E**

Run:

```bash
mise exec -- env CGO_ENABLED=1 go test -race -tags=integration ./internal/daemon -run '^TestDaemonE2EExtensionPublishedAgentSessionCommandsAndPrompt$' -count=1 -v
```

Expected: PASS; the extension-published agent session lists and invokes the real config tool and
receives the exact non-contradictory message.

- [ ] **Step 4: Update the official skill**

Document that an absent key yields `config_path_not_found` with `config path not found`, after which
the caller may set and structurally reread the same path. Use no extension or workflow name.

### Task 3: Final verification and review preparation

**Files:**
- Verify all files changed by Tasks 1 and 2.

**Interfaces:**
- Consumes: final branch diff.
- Produces: reviewable local commits and evidence for a future PR closing #394.

- [ ] **Step 1: Run formatting, focused race, and contract checks**

```bash
make fmt-check
mise exec -- env CGO_ENABLED=1 go test -race ./internal/api/core ./internal/mcp -count=1
make codegen-check
git diff --check
```

- [ ] **Step 2: Run deslop and inspect the full diff**

Confirm no duplicated formatter, prompt workaround, generated contract churn, or unrelated refactor
entered the branch.

- [ ] **Step 3: Build and run the repository gate**

```bash
mise exec -- make build
mise exec -- make gate-full
make gate-status
```

Record exact failures without weakening tests; compare suspected baseline failures against a clean
`upstream/main` worktree.

- [ ] **Step 4: Commit using repository conventions**

Stage only intended paths and create unscoped English conventional commits. Do not push or open a PR
until the user asks after reviewing the final evidence.
