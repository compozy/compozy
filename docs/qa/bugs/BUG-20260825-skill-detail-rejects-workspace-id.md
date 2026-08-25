# BUG-20260825-skill-detail-rejects-workspace-id: Every workspace-scoped skill detail and expose call is refused

- **Status:** fixed <!-- open | fixed | verified | wont-fix | invalid -->
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Dora
- **Journey Step:** J-diagnose-skill-sources, ask why a skill is missing in one workspace
- **Scenarios:** ET-skill-origin-attribution; ET-skill-source-agent-parity; ET-skill-exposure-lifecycle; ET-skill-source-diagnostics-cli
- **Found:** 2026-08-25 · **Report:** docs/qa/reports/2026-08-25-skill-sources.md

## Summary

Dora cannot inspect or expose a skill in any workspace. `compozy skill info`, `compozy skill where`,
`compozy skill expose` and `compozy skill unexpose` all refuse with `workspace_id must be the
canonical workspace id` — and they refuse **every** form of `--workspace`, including the exact
canonical `ws_…` id that `compozy workspace info` just printed. There is no value that works. The
same refusal reaches `GET /api/skills/{name}` and the expose/unexpose routes on both HTTP and UDS, so
no client can work around it.

The give-away is inside the failing response itself: the expose envelope echoes
`"workspace_id":"ws_d571d15f2795f176"` at the top level while the validator rejects that same string
as not canonical.

## Reproduction

- **Charter:** CH-skill-sources-diagnostics-truth · **Tour:** Garbage Tour
- **Environment:** desktop / wifi-fast / en-US · isolated lab daemon `127.0.0.1:55384`, build `80f17b536`

1. `compozy workspace add <path> --name skillsrc -o json` → `ws_d571d15f2795f176`
2. `compozy config set --scope workspace --workspace ws_d571d15f2795f176 skills.custom_sources <dir>`
3. `compozy skill list --workspace ws_d571d15f2795f176 -o json` → **works**, lists the skills
4. `compozy skill info audit --workspace ws_d571d15f2795f176`
5. `compozy skill where audit --workspace ws_d571d15f2795f176`
6. `compozy skill expose audit --to agents --workspace ws_d571d15f2795f176`

**Expected:** the detail, precedence and expose surfaces accept the same workspace reference the list
surface accepts, per `_dx.md` ("the CLI accepts ID, name, or path via `--workspace` and resolves
before calling. Responses echo the resolved `workspace_id`").
**Actual:** steps 4-6 all fail with `skill validation error: workspace_id must be the canonical
workspace id`. Passing the workspace *name* or *path* fails identically. Driving
`GET /api/skills/audit?workspace_id=ws_d571d15f2795f176` by hand fails the same way, so it is the
daemon, not the CLI.

Second defect on the same path: the expose route wraps this workspace-validation failure into the
per-target code `expose_target_invalid` ("expose targets are presets; custom sources cannot receive
links"), which is the wrong code and points a client at the wrong cause.

## Evidence

- `<lab>/qa-artifacts/qa/ch4/` — the walk transcripts; `skill list` succeeding beside `skill info`
  and `skill where` failing on the identical `--workspace` value
- Failing expose envelope echoing the public id it simultaneously rejects:
  `{"error":{"code":"expose_failed",…},"workspace_id":"ws_d571d15f2795f176","results":[{"target":"agents","ok":false,"error":{"code":"expose_target_invalid","message":"skill validation error: workspace_id must be the canonical workspace id"}}]}`
- Independent read path: `compozy workspace info ws_d571d15f2795f176` returns
  `id: ws_d571d15f2795f176`, confirming the rejected value is the registered id.

## Fix

- **Root cause:** *Symptom* — the public workspace id is rejected as not canonical.
  *Cause* — `canonicalResolvedWorkspaceID` (`internal/api/core/skill_exposures.go`) preferred
  `ResolvedWorkspace.WorkspaceID`, which the resolver stamps with the **durable** identity from
  `<root>/.compozy/workspace.toml` (`internal/workspace/resolver.go:149-153`), over
  `ResolvedWorkspace.ID`, which holds the registered `ws_` id every public surface hands out. The
  comparison therefore measured the caller's public id against an identity no public surface ever
  emits, so it could never match. Same family as
  BUG-20260803-agent-workspace-id-disagrees, which fixed the agent surfaces; the skill detail and
  expose routes carried the same confusion and were never covered.
  *Why the suite missed it:* the existing test
  (`TestGetSkill/Should resolve workspace-only skills from the canonical workspace id`) builds a stub
  `ResolvedWorkspace` that leaves `WorkspaceID` empty, so the fallback returned `ID` and the
  comparison passed. Production never has that shape.
- **Fix commit:** see `fix: accept the registered workspace id on skill detail and expose routes`
- **Regression test:** `internal/api/core/skills_test.go`, `TestGetSkill` — new subtest
  "Should accept the registered workspace id when a durable identity is also stamped" builds the
  resolver's real shape (both `ID` and `WorkspaceID` populated). Failed before the fix with the exact
  production message; passes after.

## Verification

- **Retested:** 2026-08-25, same persona/journey · **Report:** docs/qa/reports/2026-08-25-skill-sources.md
- **Result:** against a rebuilt lab daemon, `compozy skill info audit --workspace ws_d571d15f2795f176`
  returns the definition, and `compozy skill where` returns the winner plus both shadows with their
  qualified-form hints (`collide-a:audit`, `team-skills:audit`). `GET /api/skills/audit?workspace_id=…`
  returns the payload with `origin: "collide-b"` and `owner_scope: "workspace"`.
