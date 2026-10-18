# BUG-20260928-agent-resource-drops-fallback-chain: Continue with a declared route fails with "declares 0 route(s)"

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-14 derive branch: Continue dialog with Route n chosen (also `compozy session continue --route n`)
- **Scenarios:** ET-web-session-continue; ET-cli-session-continue
- **Found:** 2026-09-28 · **Report:** docs/qa/reports/2026-09-28-session-continue-fork-exec-b2.md

## Summary

Bruno picks an agent that declares two fallback routes. The Route select lists both, and `agent info` shows both. He chooses Route 2 and presses Continue. The dialog answers `agent "route-agent" declares 0 route(s); route 2 does not exist` and nothing is created. The CLI (`--route 1`) fails the same way.

## Reproduction

1. `compozy agent create route-agent --provider acpmock --model m --fallback-route "provider=acpmock,model=m,command=<a>" --fallback-route "provider=acpmock,model=m,command=<b>"`.
2. Open any user session → overflow → Continue with another agent… → Agent route-agent → Route 2 → Continue.

**Expected:** a child bound to the declared route. **Actual:** `route_not_found` with 0 routes.

## Root cause

`validateAgentResourceSpec` (`internal/config/agent_resource.go`) rebuilds `AgentDef` field by field and omitted `FallbackChain`. Every agent read through the resource catalog (derive target resolution) lost its chain. The agent API reads another path, so it still showed the routes.

## Fix

Added `FallbackChain: normalizeRoleFallbacks(spec.FallbackChain)` (uncommitted, task_08 B2). Regression: `TestAgentResourceCodecCanonicalizesTypedRecordSpec` asserts both routes survive decode+validate (`internal/config/agent_resource_test.go`). Re-walked: Continue with Route 2 created `sess-b946f53737c279ea`, and the child answered on the route fixture.

## Evidence

- docs/qa/evidence/2026-09-28-session-continue-fork-b2/postcommit-422-error.png (first attempt), docs/qa/evidence/2026-09-28-session-continue-fork-b2/continue-route-child-window.png (after fix), docs/qa/evidence/2026-09-28-session-continue-fork-b2/journey-log.jsonl.
