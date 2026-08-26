# BUG-20260825-skill-source-profile-write-rejected: Setting a skill source for one profile fails with an internal decoder error

- **Status:** verified <!-- open | fixed | verified | wont-fix | invalid -->
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-absorb-skills-from-other-tools, set the source policy for one profile
- **Scenarios:** ET-manage-skill-source-policy; ET-skill-source-agent-parity
- **Found:** 2026-08-25 · **Report:** docs/qa/reports/2026-08-25-skill-sources.md

## Summary

Dora cannot set `skills.sources` or `skills.custom_sources` for a named profile. Every attempt — set
and unset alike — fails with `settings validation error: decode skills settings request: unknown
field "override"`, an internal decoder message that names a field she never typed and gives her no
next step. The documentation she is following (`skills/compozy/references/configuration.md`) tells
her this exact command works at "user, exact profile (`--profile <name> --scope profile`), or
workspace scope". Two of the three scopes work; the profile one is dead.

## Reproduction

- **Charter:** CH-skill-sources-agent-plane · **Tour:** Feature Tour (headless probe, ordered first)
- **Environment:** desktop / wifi-fast / en-US · isolated lab daemon `127.0.0.1:55384`, build `80f17b536`

1. `compozy profile create helix`
2. `compozy config set --scope profile --profile helix skills.sources agents`
3. `compozy config set --scope profile --profile helix skills.custom_sources ~/team-skills`
4. `compozy config unset --scope profile --profile helix skills.sources`

**Expected:** the profile layer is written and the change applies live, exactly as it does at user
scope (`Lifecycle: live`, `Applied: true`) and workspace scope.
**Actual:** steps 2, 3 and 4 all exit 65 with
`error: cli: apply "skills.sources" via daemon settings surface: settings validation error: decode
skills settings request: unknown field "override"`. Nothing is written.

Controls that isolate the fault to *profile scope × the two source keys*:

| Case | Result |
|---|---|
| `config set --scope profile --profile helix skills.poll_interval 5s` (other skills key) | works — writes `profiles/helix/config.toml` |
| `config set --scope user skills.sources agents,claude` | works — `Lifecycle: live`, `Applied: true` |
| `config set --scope workspace --workspace <ws> skills.sources agents` | works — `Lifecycle: live`, `Applied: true` |
| `config set --scope profile --profile helix skills.sources agents` | **fails** |

The daemon is not at fault. Driving the same public route by hand shows the profile lane is
implemented and working — only the body shape the CLI sends is wrong:

```
PATCH /api/settings/skills?scope=profile&profile=helix   {"override":{...}}  -> 400 unknown field "override"
PATCH /api/settings/skills?scope=profile&profile=helix   {"config":{...}}    -> 200, config.sources=["agents","claude"]
```

## Evidence

- `<lab>/qa-artifacts/qa/probe0/04-profile-scope.txt` — first failure, with the profile created in the same transcript
- `<lab>/qa-artifacts/qa/probe0/05-profile-write-repro.txt` — the four-case control matrix above
- Independent read path: `GET /api/settings/skills?scope=profile&profile=helix` over HTTP returned
  the profile layer unchanged after each failed CLI attempt, confirming nothing was written; the
  same route with a `{"config":…}` body succeeded and the value survived a fresh read.

## Fix

- **Root cause:** *Symptom* — the CLI's profile-scope write is refused as an unknown field.
  *Cause* — the shared API decoder never offered the override shape to the profile lane.
  `internal/settings.updateSkillsSection` already accepts a `SkillSourcesOverride` at
  `ScopeProfile` (`section_skills_update.go:44`, which passes `req.ProfileName` straight into
  `updateScopedSkillSources`), and the CLI correctly sends that shape for both keys at profile scope
  — the presence-aware override is what gives `config unset` its "clear this key, inherit again"
  semantics. But `decodeSettingsSkillsUpdate` routed to the override decoder only when
  `scope == ScopeWorkspace`, so a profile-scope body fell through to the config-only branch, which
  rejects every key except `config`. The profile half of the four-layer contract was unreachable
  from any transport, HTTP and UDS alike.
- **Fix commit:** `84913fa33`
- **Regression test:** `internal/api/core/settings_test.go`,
  `TestUpdateSettingsSkillsSourcePolicyShapes` — three new subtests covering the presence-aware
  override states at `scope=profile` (set, `null`-clear, empty list), one pinning that a full
  config body still works at profile scope, and one pinning the forbidden-field refusal there.
  All four override subtests failed before the fix with the exact production message
  (`unknown field "override"`) and pass after.

## Verification

- **Retested:** 2026-08-25, same persona/journey · **Report:** docs/qa/reports/2026-08-25-skill-sources.md
- **Result:** against a rebuilt lab daemon, `compozy config set --scope profile --profile helix
  skills.sources agents,claude` wrote `profiles/helix/config.toml` with `Lifecycle: live`,
  `Applied: true`; `skills.custom_sources` did the same; `config unset` reported `deleted: true`
  and live apply; and a fresh `config get` afterwards returned the inherited user value
  `["agents","claude"]`, proving the override cleared rather than blanked the key. Evidence:
  `<lab>/qa-artifacts/qa/probe0/07-profile-fix-verify.txt`.
