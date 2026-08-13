# QA Run Report — 2026-08-13 — extension-agent-session-skills

- **Scope:** Extension-published agent session skill resolution and deterministic missing-config diagnostics across prompt, command catalog, hosted MCP, CLI, HTTP, UDS, and Web surfaces.
- **Cadence tier:** targeted
- **Build:** `v0.3.0-beta.13-14-g36bd8156-dirty` · **Environment:** fresh isolated lab, manifest `/home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Started:** 2026-08-13T12:29:50Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-session-command-catalog-parity |
| Ada | Power User | desktop / wifi-fast / en-US | CH-managed-session-skill-loading, CH-codex-native-tools-cgo0 |

## Flows in Scope

- `J-use-session-slash-commands` — discover and use the effective command catalog without losing authored text (`../journeys/J-use-session-slash-commands.md`)
- `J-load-skill-in-managed-session` — load an extension-published agent skill through hosted MCP (`../journeys/J-load-skill-in-managed-session.md`)
- `J-validate-compozy-hard-cut` — invoke canonical native tools without a legacy fallback (`../journeys/J-validate-compozy-hard-cut.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-command-catalog-parity | J-use-session-slash-commands / ET-session-command-catalog-parity | Bruno | Feature Tour | Blocked (needs human verify) | Browser driver unavailable; CLI/HTTP/UDS/Web-proxy parity passed | |
| 2 | CH-managed-session-skill-loading | J-load-skill-in-managed-session / ET-managed-session-skill-loading | Ada | Feature Tour | Pass | | |
| 3 | CH-codex-native-tools-cgo0 | J-validate-compozy-hard-cut / ET-compozy-native-tool-invocation | Ada | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-session-command-catalog-parity — Bruno

- **Ran:** 2026-08-13T12:32:32Z → 2026-08-13T12:41:00Z (box respected: yes)
- **Findings:** CLI, HTTP, direct UDS, and the isolated Vite proxy returned the same 11-command catalog and revision. Both foreign-workspace reads returned 404. Rendered menu verification is blocked because the bootstrap found no browser driver.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-session-command-catalog-parity → blocked-verify
- **Paper cuts:** none
- **Surprises:** the Web proxy was briefly 502 during Vite startup, then returned the exact daemon payload after readiness.
- **Suggested next charter:** rerun only the rendered command-menu leg when `browser-use` or `agent-browser` is installed.

### CH-managed-session-skill-loading — Ada

- **Ran:** 2026-08-13T12:32:32Z → 2026-08-13T12:35:36Z (box respected: yes)
- **Findings:** the published `reviewer` agent received all nine extension skills and used hosted MCP list/search/view without `agent not found`; the managed CLI guard remained closed.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-managed-session-skill-loading → pass
- **Paper cuts:** none
- **Surprises:** none
- **Suggested next charter:** retain the older delayed-bind TTL charter as its own regression lane.

### CH-codex-native-tools-cgo0 — Ada

- **Ran:** 2026-08-13T12:34:16Z → 2026-08-13T12:37:26Z (box respected: yes)
- **Findings:** hosted MCP exposed and executed command, skill, and config tools. Missing config produced `config_path_not_found`; workspace set and independent reread returned `false`; `todo 1.0.0` remained literal.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-compozy-native-tool-invocation → pass
- **Paper cuts:** none
- **Surprises:** none
- **Suggested next charter:** none for this diff.

## What Was Fixed

No fixes were made during this QA run.

## Paper Cuts

None.

## Runtime Errors Observed

The initial Vite proxy request returned 502 while the server was still starting; the next public request returned 200 with exact catalog parity. No product bug was filed.

## Human Verifications Needed

- [ ] Open the isolated Web session command menu with a supported browser driver and confirm the nine `/dev-cycle:*` commands render after refresh (row #1).

## Decisions for a Human

None.

## Learnings

- The pre-existing `E2E-023` bundle-size canary fails on both baseline and branch with identical bundles; this run records that limitation without changing the threshold and does not treat it as evidence about the behavior under test.
- A fresh feature-profile bootstrap includes task/channel minimums broader than this session/tool-focused diff; the strict auditor result is recorded verbatim rather than padded with synthetic activity.

## Final Status

- **Exit gate (full automated suite):** pre-existing cached full gate evidence: `.cache/gate/logs/full-1786621883.log` (reported green before this delegated QA phase; no gate command run here).
- **Strict evidence audit:** FAIL with 6 blockers: the feature-profile contract required four actors, three roles, three channels, one task root, one task run, and one auditor-recognized completed disruption probe. The focused walk exercised two actors, one role, no channels or task tree, and preserved the real HTTP/UDS workspace-fence artifacts without inventing contract-padding activity. See `qa/qa-audit-report.md` under the manifest output path.
- **Teardown:** PASS — `/home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/teardown.json` records `"clean": true`, no survivors, and closed daemon/Web PIDs.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 3 / 3 scenarios walked; one rendered-browser leg blocked by missing driver
- **Verdict:** ready with blocked items — provider, hosted MCP, config, CLI, HTTP, UDS, and Web-proxy behavior passed; a human/browser-driver re-walk is still required before claiming rendered Web parity.
