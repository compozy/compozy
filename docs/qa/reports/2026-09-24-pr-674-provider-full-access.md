# QA Run Report — 2026-09-24 — PR 674 provider full access

- **Scope:** Provider-neutral ACP full-access preference and restricted session policy.
- **Cadence tier:** targeted
- **Build:** local branch `d0557df045c24749a916480ce4c747ff7716f202` plus uncommitted Greptile remediation · **Environment:** isolated lab `pr-674-full-access-live-20260924-173054-347489`, Colima profile `pr674`, owned PostgreSQL container `compozy-pr674-pg`.
- **Started:** 2026-09-24T17:30:54Z · **Status:** PASS

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Autonomous agent | desktop / wifi-fast / en-US | CH-provider-full-access-mode |

## Flows in Scope

- `J-15` — Operate a session through structured CLI surfaces (`../journeys/J-15-operate-session-via-cli-api.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-provider-full-access-mode | J-15 / RT-provider-full-access-mode | Ada | Feature Tour | Pass | | Greptile remediation, pending commit |

## Session Debriefs

Ada used the public `compozy` CLI against a disposable workspace and daemon. `agent info` resolved Codex to `agent-full-access` and Claude Code to `bypassPermissions`. New sessions for both providers ran read-only commands against the owned `compozy-pr674-pg` Docker container and returned `compozy_qa|674` from PostgreSQL. Codex reached the database directly; Claude used `docker exec ... psql` after its host environment lacked the `psql` executable. The raw CLI results are in the lab's `qa/codex-prompt.json`, `qa/claude-prompt.json`, and `qa/claude-prompt-db.json`.

The feature tour exercised an agent-authored `read-only` Codex mode, an agent-authored `bypassPermissions` Claude mode under `approve-reads`, an explicit prompt override, and a Gemini agent. `qa/narrow-agent.json` selects `read-only` and its session replied `READY`. The restricted Claude session replied `READY` without the inherited unrestricted mode; the explicit `--acp-option mode=bypassPermissions` request failed with exit 69 and `requires approve-all permissions`. Unsupported Gemini session creation failed with exit 69 and `permissions.provider_full_access is unsupported for provider "gemini"`. All four started sessions stopped with `verified: true`.

## What Was Fixed

The session-start regression caught by Greptile was repaired in `internal/session`: a restricted session drops an authored unrestricted agent mode before ACP start. Existing prompt rejection remains in place. See the owning Go session tests and the CLI receipts in this report.

## Paper Cuts

Claude's host shell did not contain `psql`; the operator used the PostgreSQL client inside the owned container. This was an environment limitation, not a Compozy failure.

## Runtime Errors Observed

No Compozy runtime errors in the final provider prompts. The expected restricted override and unsupported-provider errors were observed.

## Human Verifications Needed

None for this targeted journey.

## Decisions for a Human

None.

## Learnings

`agent info` reports authored options, while the provider's effective option is evidenced by the running session and provider command results. Session-start policy must be checked after runtime resolution, since resolution can reintroduce an authored unrestricted mode.

## Final Status

**PASS** for the targeted J-15 / RT-provider-full-access-mode journey: both live providers reached owned Docker and PostgreSQL fixtures, the policy edges behaved as specified, and all sessions stopped. No open QA bug or human verification remains for this scenario. Evidence: `/Users/pedronauck/dev/qa-labs/compozy-pr-674-full-access-live-20260924-173054-347489-lab/qa-artifacts/qa/` (`journey-log.jsonl`, `provider-attempt.json`, prompt receipts, and strict auditor). PR delivery still depends on the final local gate and exact-head CI.

Local `make gate` PASS: `.cache/gate/go-lint.json` and `.cache/gate/go-test.json` (affected Go lint and race suites). The integration-tagged extension E2E and final exact-head CI remain separate PR delivery checks.

The targeted lab's strict `audit-qa-evidence.py --strict` returned no blockers or warnings. Its `qa/qa-audit-report.json` and `qa/teardown.json` are retained under the lab evidence path; teardown reports `clean: true` and no survivors. The owned PostgreSQL container and Colima `pr674` profile were removed after the walk.

After this live walk, the session prompt runtime suite caught and verified one adjacent edge: a restricted session can change an ACP runtime option while keeping the full-access mode filtered. The paired test still refuses a full-access preference enabled only after session creation. This edge has focused race evidence in `internal/session/manager_transition_test.go`; the lab had already been torn down and was not rerun for that unit-level distinction.
