# QA Run Report — 2026-08-26 — Main Rebase Regressions

- **Scope:** regressions exposed while rebasing PR #488 onto the profile-identity work merged by PR #484.
- **Cadence tier:** targeted
- **Build:** rebased head plus the current reviewed working tree
- **Environment:** fresh isolated targeted lab pending
- **Started:** 2026-08-26
- **Status:** in progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Autonomous Agent | laptop / wifi-fast / en-US | CH-session-skill-catalog-budget |
| Iris | Remote Operator | laptop / wifi-slow / en-US | CH-gateway-remote-cli-interruption |

## Flows in Scope

- `J-use-absorbed-skills-in-a-session` — keep a large injected catalog complete inside its budget.
- `J-operate-remote-gateway-cli` — classify an interrupted accepted prompt stream without parsing prose.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Finding or evidence |
|---|---|---|---|---|---|---|
| 1 | CH-session-skill-catalog-budget | J-use-absorbed-skills-in-a-session / ET-session-skill-catalog-budget | Ada | Feature Tour | Pending | Fresh managed-session walk pending |
| 2 | CH-gateway-remote-cli-interruption | J-operate-remote-gateway-cli / RT-gateway-remote-cli-profile | Iris | Interrupt Tour | Pending | Authorized remote Gateway pending |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Human Verifications Needed

- The remote-profile interruption leg needs an authorized HTTPS Gateway address and a paired device.
  No external provider address is available in this workspace, so the automated exact-contract E2E
  remains the backstop until a human can perform that live walk.

## Compozy Impact Audit

- **Native tools:** pending final diff and generated-descriptor check.
- **Extensibility and hooks:** pending final config, skill-resource, and hook check.
- **Workspace data isolation:** pending final session/workspace propagation check.
- **Official Compozy skill:** pending final bundled-reference check.

## Final Status

Pending the targeted persona walk, fresh local gate, clean teardown evidence, and exact-head PR CI.
