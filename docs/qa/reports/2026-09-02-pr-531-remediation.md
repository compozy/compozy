# QA Run Report — 2026-09-02 — PR 531 remediation

- **Scope:** Re-verify Profile-scoped lifecycle tool discovery and tool-gated skill activation after PR review remediation.
- **Cadence tier:** targeted
- **Base:** `d28940cd9c724b38029750ec57a8d33ce1f7a917` · **Build:** `ee410b18fe959e29a60140ed1020448859247070` plus remediation tree fingerprint `2fc26e71c46f59558ca1a5e31d20017aad1a457a` · **Environment:** manifest-isolated CLI/UDS, HTTP, runtime, and extension-provider lab
- **Started:** 2026-09-02T17:00:00Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Developer/operator | desktop / wifi-fast / en-US | `CH-profile-scoped-loop-tools` |
| Ada | Capability administrator | desktop / wifi-fast / en-US | `CH-profile-tool-gated-skills` |

## Flows in Scope

- `J-complete-partial-loop` — lifecycle action schema discovery stays inside the acting workspace and Profile.
- `J-offer-runnable-capabilities` — named-Profile tool availability controls the next skill catalog without a restart.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | `CH-profile-scoped-loop-tools` | `J-complete-partial-loop` / `LP-extension-action-schema-scope` | Bruno | Feature Tour | Pass | | `ee410b18` |
| 2 | `CH-profile-tool-gated-skills` | `J-offer-runnable-capabilities` / `ET-skill-activation-gates` | Ada | Feature Tour | Fixed | COR-001 | remediation tree |

## Session Debriefs

### CH-profile-scoped-loop-tools — Bruno

- **Ran:** 2026-09-02T17:08:44Z → 2026-09-02T17:16:00Z (box respected: yes)
- **Findings:** None. CLI/UDS and HTTP accepted `ext__qa_lab__search` only in `qa-profile` and workspace `ws_03fc55f1d6457d6c`; the default Profile and peer workspace returned `unknown_action_kind`.
- **Bugs filed/updated:** None.
- **Scenarios settled:** `LP-extension-action-schema-scope` → `pass`.
- **Paper cuts:** None.
- **Surprises:** A numeric action parameter is lint-valid because runtime binding owns tool input validation; external tool invocation remained hidden by the default source policy, as designed.
- **Suggested next charter:** Re-run extension-owned action execution when a disposable bundled Loop fixture is available.

### CH-profile-tool-gated-skills — Ada

- **Ran:** 2026-09-02T17:10:00Z → 2026-09-02T17:16:30Z (box respected: yes)
- **Findings:** COR-001 was fixed before the walk. The named-Profile catalog offered `profile-tool-gated`; the default Profile reported `missing_tool`; disable and enable changed the next projection without daemon restart.
- **Bugs filed/updated:** None; COR-001 is a reviewer finding, not a new QA symptom.
- **Scenarios settled:** `ET-skill-activation-gates` → `pass`.
- **Paper cuts:** None.
- **Surprises:** None after the required provider defaults were configured through the public config writer and the daemon restarted.
- **Suggested next charter:** Exercise the same gate from a live managed-session prompt when a disposable provider session is in scope.

## What Was Fixed

### COR-001: Profile-scoped tools were omitted from skill activation

- **Symptom:** A skill gated by an extension tool disappeared in a named Profile even though that Profile owned the available tool.
- **Root cause:** The resolved Profile ID was dropped from `ActivationTarget`, then omitted from the daemon tool registry scope.
- **Fix:** Propagate the normalized Profile ID through workspace and agent skill projections into the runtime tool registry lookup.
- **Regression test:** `internal/daemon/prompt_skills_test.go` — failed before with `profile id = "", want "profile-work"`; passes after the fix.
- **Retested:** Both targeted journeys were walked from fresh public reads; the Loop scope journey also covered the adjacent extension registry path.

## Paper Cuts

None recorded.

## Runtime Errors Observed

- The generated extension initially lacked a local SDK replace and `go.sum`; the fixture was aligned with the repository's external-consumer test before the session.
- Profile provider defaults require a daemon restart. The public config writer reported that lifecycle explicitly, and the session restarted before collecting verdict evidence.
- `config set tools.policy.external_default` is intentionally unsupported; the lab preserved the default external-source policy and did not hand-edit config.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Skill activation needs the same Profile identity used by extension placement; workspace/session/agent identity alone is insufficient.
- A targeted extension fixture can prove Profile placement without provider credentials or global config mutation.

## Compozy Impact Audit

- **Native tools:** No native tool ID, toolset, descriptor, schema digest, risk flag, availability diagnostic, capability gate, or CLI/API fallback changed. Checked the shared runtime registry projection used by native skill catalogs and Loop tools.
- **Extensibility and hooks:** Profile identity now reaches skill activation lookups for extension-contributed tools. Checked extension placement, resources, runtime registry projections, cache identity, disable/enable refresh, and config lifecycle; no hook event, bridge SDK, MCP sidecar, extension resource ID, or `config.toml` key/default changed.
- **Workspace data isolation:** Activation input is ephemeral and scoped by Profile, workspace, session, and agent. Checked propagation through resolved workspace → skill activation target → daemon tool scope, plus CLI/HTTP/UDS Loop validation and peer-workspace denial; no durable store, SSE event, or cache key changed.
- **Official Compozy skill:** No update required. Checked `skills/compozy/`; no public tool ID, CLI path, hook event, capability, extension resource, memory/network/task semantic, or operator workflow changed.

## Final Status

- **Exit gate:** PASS — `make gate`, fingerprint `2fc26e71c46f59558ca1a5e31d20017aad1a457a`; Go lint log `.cache/gate/logs/go-lint-1788369526-28708.log`, race-test log `.cache/gate/logs/go-test-1788369547-28708.log`.
- **Strict evidence audit:** PASS — `.cache/qa-labs/compozy-pr-531-profile-scopes-20260902-170122-478233-lab/qa-artifacts/qa/qa-audit-report.json` reports no blockers or warnings.
- **Teardown:** `.cache/qa-labs/compozy-pr-531-profile-scopes-20260902-170122-478233-lab/qa-artifacts/qa/teardown.json` reports `clean: true` with no survivors.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0.
- **Coverage:** 2/2 targeted journeys walked through CLI/UDS, HTTP, runtime restart, and a real subprocess extension provider.
- **Verdict:** PASS — the remediation tree passed strict evidence audit and clean teardown; exact-head PR CI remains the delivery gate.
