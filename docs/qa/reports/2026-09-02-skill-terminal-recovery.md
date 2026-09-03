# QA Run Report — 2026-09-02 — Skill and Terminal Recovery

- **Scope:** actionable `skill_view` failures, malformed-skill catalog containment, and bounded recovery from a failed Compozy terminal reference read.
- **Cadence tier:** targeted journey using the feature-profile evidence contract
- **Build:** `c102779c30cf01218864a926a0e63cee668c853c` plus the current working-tree fix · **Environment:** fresh isolated lab `skill-terminal-recovery-20260902-205559-939126`
- **Started:** 2026-09-02T20:52:21Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Power User | desktop / wifi-fast / en-US | CH-malformed-skill-recovery |
| Ada | Power User | desktop / wifi-fast / en-US | CH-skill-view-error-recovery |
| Bruno | Power User | desktop / wifi-fast / pt-BR | CH-terminal-reference-recovery |

## Flows in Scope

- `J-diagnose-skill-sources` — identify and repair the definition that kept a skill out of the catalog (`../journeys/J-diagnose-skill-sources.md`)
- `J-load-skill-in-managed-session` — read or recover an omitted skill through the native seam (`../journeys/J-load-skill-in-managed-session.md`)
- `J-operate-integrated-terminal` — open and supervise a visible terminal without confusing internal commands for terminal state (`../journeys/J-operate-integrated-terminal.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-malformed-skill-recovery | J-diagnose-skill-sources / ET-skill-ecosystem-frontmatter-quiet | Dora | Garbage Tour | Pass | [#535](https://github.com/compozy/compozy/issues/535) | working tree |
| 2 | CH-skill-view-error-recovery | J-load-skill-in-managed-session / ET-skill-view-actionable-errors | Ada | Garbage Tour | Pass | [#535](https://github.com/compozy/compozy/issues/535) | working tree |
| 3 | CH-terminal-reference-recovery | J-operate-integrated-terminal / ET-agent-terminal-window-materialization | Bruno | Feature Tour | Pass | | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-malformed-skill-recovery

The workspace root began with two `SKILL.md` files but published only the healthy neighbor and
reported one blocked definition. Repeated source reads did not emit the former directory-name
fallback warning. After fixing the unclosed YAML list, the same daemon published both skills and
reported no blocked definitions.

Evidence: `/Users/pedronauck/dev/qa-labs/compozy-skill-terminal-recovery-20260902-205559-939126-lab/qa-artifacts/qa/test-cases/malformed-skill-recovery.md`.

### CH-skill-view-error-recovery

Managed session `sess-58ca92f149e57ac5` received `tool_invalid_input` with reason
`skill_definition_invalid`, an exact `SKILL.md` path, YAML source location, and recovery guidance
through hosted MCP. After repair, the same session returned the marker
`recovered-without-daemon-restart-20260902`. A separate missing resource returned `tool_not_found`
with `skill_resource_not_found`.

Evidence: `/Users/pedronauck/dev/qa-labs/compozy-skill-terminal-recovery-20260902-205559-939126-lab/qa-artifacts/qa/test-cases/skill-view-hosted-recovery.md`.

### CH-terminal-reference-recovery

The exact operator prompt was replayed in managed session `sess-795e2e3bb8afc603` against the same
workspace that contains the stale physical `.agents/skills/compozy` copy. `/agents:compozy` resolved
to `skill:bundled:agents-727a535ff8d83def:compozy:qualified`; all three required resources loaded,
including `references/terminal.md`. The agent opened visible terminal `term-2b7ed0bb80ea`, ran two
safe command sequences, waited for idle output, and yielded the lease to the operator. The unchanged
close/reopen and process-lifetime portion keeps the full prior evidence in
`docs/qa/reports/2026-09-01-terminal-rework.md`.

Evidence: `/Users/pedronauck/dev/qa-labs/compozy-skill-terminal-recovery-20260902-205559-939126-lab/qa-artifacts/qa/test-cases/terminal-prompt-replay.md`.

## What Was Fixed

- Malformed definitions are withheld instead of published under a directory-name fallback.
- `skill_view` distinguishes an absent resource from malformed frontmatter with stable reason codes.
- Hosted MCP preserves separately marked operator diagnostics instead of collapsing them into a generic backend failure.
- The official Compozy skill retries one exact reference read and allows only a descriptor-complete, one-command visible terminal demonstration after the second failure.
- Slash-command activation binds every referenced read to the exact opaque `command_id`.
- The bundled `compozy` runtime skill is reserved: external copies keep their qualified command token but resolve the daemon's current bundled body and resources.

## Paper Cuts

The browser-only shell cannot represent the desktop client's live-layout connection; this is an
environment boundary, not a regression in the changed terminal or skill paths.

## Runtime Errors Observed

No unexpected runtime errors. The malformed definition produced the expected typed invalid-input
result before repair.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- A malformed definition must remain addressable through diagnostics even after it is correctly withheld from the effective catalog.
- Error transport must preserve safe primary text and explicit operator detail as separate fields.
- The ordinary prompt is a stronger terminal canary than directly instructing the agent which tool to call.
- Prompt warnings cannot safely repair a stale control-plane skill; source resolution must bind qualified aliases to the daemon-owned contract.

## Compozy Impact Audit

- **Native tools:** `compozy__skill_view` gained typed missing-resource and invalid-definition reasons, an exact-path descriptor hint, generated schema updates, catalog digest refresh, and daemon/API/hosted-MCP coverage.
- **Extensibility and hooks:** skill discovery, command aliases, registry diagnostics, and hosted MCP error transport changed; external `compozy` copies can no longer replace the daemon-owned runtime contract. Extension registries, hooks, sidecars, and `config.toml` lifecycle were checked and are otherwise unchanged.
- **Workspace data isolation:** malformed-definition diagnostics remain resolved through the caller's profile/workspace/agent scope; no new persisted datum, cache, SSE shape, or event ownership was added.
- **Official Compozy skill:** `skills/compozy/SKILL.md` defines the exact retry and bounded descriptor fallback; `references/tools-and-skills.md` documents reserved runtime-skill alias behavior.

## Final Status

- **Exit gate (scoped local gate):** pass; all current lanes passed before the final report-only update (`f16905205205242a94fd103044437b54806cd1fcd42`)
- **Issues by user impact:** all targeted retests pass; GitHub #535 remains fixed in the working tree
- **Coverage:** 3/3 journeys pass
- **Teardown:** pass; `qa/teardown.json` reports `clean: true` with no survivors.
- **Verdict:** pass.
