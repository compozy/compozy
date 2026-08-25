# BUG-20260825-workspace-agent-unusable-for-sessions: An agent the catalog lists cannot start a session

- **Status:** open <!-- open | fixed | verified | wont-fix | invalid -->
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-workspace-context, create a workspace agent then use it
- **Scenarios:** ET-managed-session-skill-loading
- **Found:** 2026-08-25 · **Report:** docs/qa/reports/2026-08-25-skill-sources.md

## Summary

An agent created with `compozy agent create <name> --workspace <ws>` is reported as created, is
returned by `compozy agent list`, and is fully readable through `compozy agent info` — including a
resolved `effective_runtime` with a provider and model. It then cannot be used: `compozy session new
--agent <name> --workspace <ws>` refuses with `agent not available in workspace`. Every read surface
says the agent exists and is ready; the one verb that would use it says it does not exist. Ada has no
way to tell from the catalog which listed agents are actually usable.

Found while standing up the runtime lane for this cycle; it is not a skill-sources defect, but it
blocked the playbook lane until the agents were re-registered at the global layer.

## Reproduction

- **Charter:** (found during lab setup for CH-skill-sources-managed-session-canary) · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US · isolated lab daemon `127.0.0.1:55384`, build `80f17b536`, after `compozy install --provider claude --model claude-sonnet-5`

1. `compozy workspace add <path> -o json` → `ws_1a9b181e45528661`
2. `compozy agent create eng-lead-agent --workspace ws_1a9b181e45528661 --provider claude --prompt-file <file> -o json` → succeeds, `origin: workspace`, `layer: project_profile`
3. `compozy agent list --workspace ws_1a9b181e45528661 -o json` → the agent is listed
4. `compozy agent info eng-lead-agent --workspace ws_1a9b181e45528661 -o json` → returns the full definition with `effective_runtime.provider: claude`, `model: claude-sonnet-5`
5. `compozy session new --agent eng-lead-agent --workspace ws_1a9b181e45528661 -o json`

**Expected:** a session starts on the agent every read surface just described as available.
**Actual:** `{"error":"session: resolve create runtime for \"sess-…\": session: resolve workspace agent \"eng-lead-agent\": agent not available in workspace: eng-lead-agent"}`

Controls that isolate the fault to the workspace-profile layer:

| Case | `origin` / `layer` | `session new` |
|---|---|---|
| `agent create <name> --workspace <ws>` (all 8 playbook agents) | `workspace` / `project_profile` | **fails** |
| `agent create <name>` with no `--workspace` | `global` / — | succeeds |
| built-in `general` after `compozy install` | `global` / `user` | succeeds |

Not a provider-configuration problem: before `compozy install` the `general` control failed with a
*different, accurate* error (`agent provider is required`), and after `install` it succeeded while the
workspace agents kept failing with the availability error. Not a current-profile problem either: the
failure is identical under the `helix` profile, under `--profile helix`, and after switching back to
`default` — while `agent list` keeps showing all eight in every case.

## Evidence

- `<lab>/qa-artifacts/qa/setup/operator-session.json` — first refusal
- Definitions landed at `<ws>/.compozy/profiles/default/agents/<agent-id>/`, i.e. the
  workspace-profile layer, while `session new` resolves against `ResolvedWorkspace.Agents`
  (`internal/session/manager_workspace.go:206-224`), which does not carry that layer.
- Independent read path: `compozy agent info` and `compozy agent list` both continued to advertise
  every one of the eight agents after each failed session attempt.

## Fix

<!-- Not auto-fixed — see Decisions for a Human in the run report. -->
- **Root cause (suspected, not confirmed by a fix):** `compozy agent create --workspace` writes to the
  workspace-profile layer (`<ws>/.compozy/profiles/<profile>/agents/`), but the workspace resolver
  that `session new` consults (`resolveWorkspaceAgent` over `resolvedWorkspace.Agents`) does not
  include that layer. Either the scanner should surface workspace-profile agents to session
  resolution, or `agent create --workspace` should write to a layer sessions can reach — that is a
  product decision, not this session's to make.
- **Fix commit:**
- **Regression test:**

## Verification

<!-- filled when status moves to verified -->
