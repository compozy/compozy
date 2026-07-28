# BUG-20260724-inherited-role-provider-resolution: Model-only inherited roles omit the invoking provider

- **Status:** verified
- **Impact (user-side):** Functional
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora; Ada
- **Journey Step:** J-route-background-work, live Settings save → next auto-title invocation
- **Scenarios:** MS-background-role-routing; MS-background-role-fallback; MS-settings-roles-panel
- **Found:** 2026-07-24 · **Report:** docs/qa/reports/2026-07-24-agent-roles.md

## Summary

A valid model-only override on the inherited `auto_title` or `memory_extractor` role left the effective provider empty. Session creation rejected the primary route before acceptance, so auto-title either failed or used a configured fallback even though the invoking agent had a valid provider.

## Reproduction

- **Charters:** CH-background-role-routing-scopes, CH-role-fallback-boundary, CH-settings-roles-live-truth
- **Environment:** desktop / wifi-fast / en-US, isolated `devtool-oss-launch` lab

1. Configure `roles.auto_title.enabled = true`, `model = "gpt-5.6-luna"`, and no role-level provider.
2. Start a `general` session on provider `codex` and complete its first turn.
3. Inspect the generated title, hidden auto-title child, and `role.fallback.used` logs.

**Expected:** The inherited role resolves `codex` from the invoking agent, starts the hidden child on `codex/gpt-5.6-luna`, and needs no fallback.
**Actual:** The primary creation spec contains `provider = ""` with `model = "gpt-5.6-luna"`; validation rejects it before acceptance and advances to the fallback when one is configured.

## Evidence

- Before: `ui-live-auto-title-session.json`, `ui-live-fallback-{cli,http}.json` in the run's `qa-artifacts/qa` directory.
- After: `inherit-provider-fix-{session-create,auto-title-child,parent-after-title,fallback-events,visible-sessions,session-stop}.json` in the same directory.

## Fix

- **Root cause:** `RoleResolver.resolveEffective` returned immediately for an inherited role before reading invocation correlation or resolving the invoking agent's provider chain.
- **Fix:** In invocation context, inherited roles now resolve the correlated agent and apply the documented agent-provider → default-provider chain while preserving the role's explicit model override. Projection reads without invocation context remain honestly unresolved.
- **Fix commit:** `a9a8fcad63f4354505e4c9a0701a6d0f559cc991`
- **Regression test:** `internal/daemon/role_resolver_test.go` — `TestRoleResolver/Should resolve inherited role overrides through the invoking agent provider chain` covers both an explicit invoking-agent provider and an invoking agent that inherits `defaults.provider`.

## Verification

- The focused regression failed red for both roles before the fix, then passed in four parallel cases; the complete daemon package passed 1,399 `-race` tests and repository lint passed with zero issues.
- A rebuilt/restarted daemon created hidden child `sess-afc891322e1060b8` as `general/codex/gpt-5.6-luna`, completed it, generated the parent title `inherited provider probe complete`, emitted zero `role.fallback.used` events, and kept the child out of the public session list.
