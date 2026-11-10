# Full access runs the provider unrestricted (#1473)

Scope: `RT-provider-full-access-mode` and the changed Full access note in `MS-web-agent-permissions`,
walked against a daemon built from the fix branch in an isolated lab (own `COMPOZY_HOME`, port,
socket and Web dev server). Persona: Ada, an operator who picked **Full access** in Settings.
Codex ran live through the operator's existing sign-in. Claude Code and Copilot were checked through
`compozy agent info` only; their live sessions were not started in this pass.

The lab config held only `[permissions] mode = "approve-all"` and the deprecated
`provider_full_access = false`, so the walk also proves the key no longer narrows anything.

| Journey | Status | Evidence |
| --- | --- | --- |
| Deprecated key loads with a warning | Pass | Daemon and each CLI process log one warning naming `permissions.mode` and the v0.5.7 removal |
| Agent resolution at approve-all | Pass | `agent info`: Codex `mode=agent-full-access`, Claude Code `mode=bypassPermissions`, Copilot `allow_all=on` |
| Agent resolution at auto | Pass | `agent info` for a Codex agent at `auto` lists no unrestricted option |
| Fresh Codex session at approve-all | Pass | Effective runtime `mode=agent-full-access`; the provider confirmed it at start; the shell reported `sandbox=[unset] net_disabled=[unset]` |
| Fresh Codex session at auto | Pass | `effective_permissions: auto`, `permission_mode_degraded: false`; the shell reported `sandbox=[seatbelt] net_disabled=[1]` (Codex `agent` reviewer mode) |
| Explicit narrower agent mode | Pass | A Codex agent authoring `mode=read-only` kept `read-only` under approve-all; Codex raised a permission request for a write |
| Explicit unrestricted selection below approve-all | Pass | `--acp-option mode=agent-full-access` on the `auto` session exited 69: `requires approve-all permissions` |
| Web agent Permissions at Full access on Codex | Pass | No provider-reviewer note; Full access selected (explicit agent) or Follow Settings naming Full access |

Before the fix the same config started Codex in `agent` (Auto review) under Full access, which is
what the issue reported: workspace-write, network off, and the Guardian reviewer stopping
`git push` and `gh issue create`.

## Investigation

`ResolveAgent` mapped `approve-all` to each provider's unrestricted ACP option only when the separate
`permissions.provider_full_access` opt-in was set. Without it, ACP mode negotiation ran the
`approve-all` candidate list, whose first match on Codex is `agent`, the same mode `auto` declares as
its reviewer. The fix applies the mapping whenever the resolved level is `approve-all`, leaves
providers without a mapping to their existing negotiation instead of failing, and keeps the agent's
explicit selection and the session-level drop and refusal unchanged.

## Rendered evidence

The Web screenshot of the Codex agent at Full access is linked from the pull request.

Unit coverage: `internal/config/provider_test.go`, `internal/config/provider_agy_test.go`,
`internal/session/manager_start_test.go`, `internal/session/manager_transition_test.go`,
`internal/api/core/agent_contracts_test.go` and
`web/src/systems/agent/components/__tests__/agent-editors.test.tsx`.

Targeted teardown reported `clean: true`. The installed application, the operator's config and
the operator daemon were not touched.

Final status: **PASS** for the targeted walk.
