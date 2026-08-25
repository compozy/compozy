# BUG-20260825-skill-source-agent-write-doc-mismatch: The official skill tells agents a config write is denied that actually succeeds

- **Status:** fixed <!-- open | fixed | verified | wont-fix | invalid -->
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-skill-sources-headless, read the reference then change source policy
- **Scenarios:** ET-skill-source-agent-parity; ET-compozy-official-skill-discovery
- **Found:** 2026-08-25 · **Report:** docs/qa/reports/2026-08-25-skill-sources.md

## Summary

The shipped Compozy skill tells an agent that `skills.sources` and `skills.custom_sources` are trust
roots which `compozy__config_set` and `compozy__config_unset` refuse with
`config_trust_root_forbidden`. They do not refuse. Both tools write both keys successfully at user
and workspace scope and report `applied: true, lifecycle: live`. An agent that believes the
reference will never attempt a write it is allowed to make, and will route the operator through a
manual step that was never necessary — and an agent that trusts the reference's error vocabulary
will match on a code the runtime never emits.

Planning found this contradiction by reading the code (`task_08.md`, "Decision for a Human"); this
file records it after reproduction through public surfaces, which is what the registry requires.

## Reproduction

- **Charter:** CH-skill-sources-agent-plane · **Tour:** Feature Tour (headless probe, ordered first)
- **Environment:** desktop / wifi-fast / en-US · isolated lab daemon `127.0.0.1:55384`, build `80f17b536`

1. `compozy tool invoke compozy__config_set --input '{"path":"skills.sources","value":["agents","claude"],"scope":"user"}' -o json`
2. `compozy tool invoke compozy__config_set --input '{"path":"skills.custom_sources","value":["<dir>"],"scope":"user"}' -o json`
3. `compozy tool invoke compozy__config_set --input '{"path":"skills.sources","value":["agents"],"scope":"workspace","workspace":"<ws>"}' -o json`
4. `compozy tool invoke compozy__config_unset --input '{"path":"skills.custom_sources","scope":"user"}' -o json`
5. `compozy tool invoke compozy__config_set --input '{"path":"skills.sources","value":["agents"],"scope":"agent"}' -o json`
6. Same call at `"scope":"profile"`.

**Expected (per the shipped reference):** steps 1-4 are denied with `config_trust_root_forbidden`.
**Actual:** steps 1-4 all succeed — `status: completed`, `applied: true`, `lifecycle: live`,
`next_action: none`, with an `apply_record_id` and a bumped `active_generation`. Steps 5 and 6 are
denied, but with `tool_denied` / `reason_codes: ["config_scope_not_allowed"]` — not
`config_trust_root_forbidden`, which never appears anywhere in this matrix.

The runtime behavior matches the binding spec, so the documentation is the defect, not the code:
`_spec.md:445` states `tool_surface` registers both keys as "agent-writable string slices at
global+workspace scope … read-only at agent scope", which is exactly what shipped.

Two shipped files carry the wrong claim:

- `skills/compozy/references/configuration.md:136-137` — "Both keys are trust roots, so
  `compozy__config_set` and `compozy__config_unset` deny them with `config_trust_root_forbidden`"
- `skills/compozy/references/tools-and-skills.md:176` — "trust roots: `compozy__config_set` denies
  them with `config_trust_root_forbidden`, so read them at …"

The site's only `config_trust_root_forbidden` reference (`config-toml.mdx:1742`) is about the
marketplace feed root and is correct — no site change is implied.

## Evidence

- `<lab>/qa-artifacts/qa/probe0/01-config-set-user.txt` — both user-scope writes applied
- `<lab>/qa-artifacts/qa/probe0/02-config-scopes.txt` — workspace applied; agent and profile denied with `config_scope_not_allowed`
- `<lab>/qa-artifacts/qa/probe0/06-agent-write.json` — agent write with `applied: true`
- Independent read paths after the agent write: `GET /api/settings/skills?scope=user` over HTTP and
  over the UDS socket both returned `["agents","claude"]`, and `compozy skill sources` showed the
  `claude` preset flipped to `enabled` — the write was real, not optimistic.

## Fix

- **Root cause:** *Symptom* — the reference promises a denial the runtime never performs.
  *Cause* — the documentation, not the code. Both paths sit in `agentMutableConfigKinds`
  (`internal/config/tool_surface.go:155-156`) and `ClassifyToolConfigPath` returns on that lookup
  (`:278-281`) before it ever reaches the trust-root branch (`:294-296`). They are also listed in
  `skillsConfigPathIsTrustRoot`, which is what the doc author read — but that check is unreachable
  for these two keys. Task 07 wrote the sentence from the unreachable branch; the runtime has always
  matched `_spec.md:445` instead.
- **Fix commit:** see `docs: correct the agent write policy for skill source keys`
- **Regression test:** documented replay — the reproduction transcript above *is* the before/after
  evidence, and prose has no red-before test. A guard was added in the canonical suite
  `internal/config/tool_surface_test.go` (`TestToolConfigPathPolicy`) pinning both paths as
  agent-mutable `ConfigValueStringSlice`, so a future reordering of the classifier cannot silently
  make the corrected sentence wrong again. That guard covers the code direction only; the missing
  doc-vs-runtime gate is filed at
  `docs/qa/automation-backlog/official-skill-doc-runtime-agreement.md`.

## Verification

- **Retested:** 2026-08-25, same persona/journey · **Report:** docs/qa/reports/2026-08-25-skill-sources.md
- **Result:** the corrected sentences in `skills/compozy/references/configuration.md` and
  `references/tools-and-skills.md` now state what the probe matrix observed — writable at user and
  workspace scope, refused at agent and profile scope with `config_scope_not_allowed`. Re-reading
  both references against the recorded transcript leaves no claim the runtime contradicts.
