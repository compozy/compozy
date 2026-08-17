# QA Run Report — 2026-08-17 — PR 423 resource-only watch

- **Scope:** PR #423 resource-only extension generation fidelity, live watch reload, workspace projection, and code-backed compatibility canary
- **Cadence tier:** targeted
- **Build:** base `2b7c07d4b725ee0b0b95a6824e230279d37feee1`; frozen product diff SHA-256 `31dc432f9e63e260c7ce2a32d2c5fda3fcbfe4af1a892fb0671096cbffd09e63`
- **Environment:** isolated CLI/API/runtime lab `compozy-pr-423-resource-only-watch-20260817-185100-670194-lab`; real built binary and daemon on isolated port 61408; provider and browser intentionally outside the targeted contract
- **Started:** 2026-08-17T15:50:21-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-resource-only-extension-watch |

## Flows in Scope

- `J-extension-dev-lifecycle` — operate immutable workspace-scoped extension generations safely (`../journeys/J-extension-dev-lifecycle.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-resource-only-extension-watch | J-extension-dev-lifecycle / ET-resource-only-extension-dev | Bruno | Feature Tour | Pass | | |
| 2 | CH-resource-only-extension-watch | J-extension-dev-lifecycle / ET-extension-dev-reload-loop | Bruno | Feature Tour | Pass | adjacent code-backed canary only; broad scenario keeps prior verdict | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-resource-only-extension-watch — Bruno

- **Ran:** 2026-08-17T15:53:38-03:00 → 2026-08-17T15:59:02-03:00 (box respected: yes)
- **Findings:**
  - A passive agent-and-skill source with no language project built as generation `898e6f20…`; both JSON output and the generated manifest retained the required Live Network block and `builders` scope.
  - The first dev attempt refused before linking and returned digest `24e0db59…`; retrying with that exact digest linked the same generation without an install trust prompt.
  - `extension dev --watch` stayed active through a `SKILL.md` edit and emitted generation `8df8e674…`. A fresh workspace skill read returned the changed description; workspace agent read succeeded; both global catalogs excluded the extension resources.
  - An invalid YAML edit stopped the watcher with a specific staged `SKILL.md` diagnostic. A fresh extension list still reported active generation `8df8e674…` with `activation_failed`, and the skill catalog retained the prior valid description.
  - The adjacent Go scaffold built, dev-linked, and returned `No results for alpha` through `ext__code_canary__search`.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-resource-only-extension-dev → pass; ET-extension-dev-reload-loop retained its prior pass and served only as an adjacent code-backed canary
- **Paper cuts:** none
- **Surprises:** the initial Network refusal and invalid-watch failure both preserved state and returned actionable structured diagnostics.
- **Suggested next charter:** concurrent watch processes in two workspaces if workspace reload coordination changes again.

## What Was Fixed

No QA-session fixes.

## Paper Cuts

None.

## Runtime Errors Observed

- Expected negative probe: missing Network confirmation returned `extension_network_confirmation_required` with the exact current digest and no dev link.
- Expected negative probe: malformed skill YAML ended the watcher while the daemon retained the last-good generation.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- The CLI watch process itself is the strongest proof for watch behavior; manually calling reload does not cover the polling boundary.
- The generated manifest and a fresh workspace catalog are complementary read paths: one proves package fidelity, the other proves daemon activation and scope.
- Taxonomy sweep: happy path, Network refusal, invalid-edit recovery, workspace/global isolation, deterministic structured output, and the code-backed regression canary were covered. Browser, mobile, locale, and provider dimensions were deliberately skipped because this charter is CLI/API/runtime-only.
- Experiential lenses: usability, perceived performance, error recovery, and production parity passed for the targeted CLI flow; accessibility and browser compatibility were not applicable to the declared surfaces.

## Final Status

- **Automated QA precondition:** `make gate` passed lint and all affected Go packages with `-race`; evidence: `/Users/pedronauck/dev/qa-labs/compozy-pr-423-resource-only-watch-20260817-185100-670194-lab/qa-artifacts/qa/verify-affected.log`.
- **Strict evidence audit:** `/Users/pedronauck/dev/qa-labs/compozy-pr-423-resource-only-watch-20260817-185100-670194-lab/qa-artifacts/qa/qa-audit-report.json`.
- **Teardown evidence:** `/Users/pedronauck/dev/qa-labs/compozy-pr-423-resource-only-watch-20260817-185100-670194-lab/qa-artifacts/qa/teardown.json`.
- **Repository close gate:** `make gate-full` runs once after QA teardown and the final repository mutation; its tree-fingerprint record, not this behavior report, owns merge readiness.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1 changed journey walked by Bruno; code-backed lane exercised as the adjacent canary; browser/provider dimensions intentionally excluded by the targeted contract.
- **Verdict:** **PASS** — the targeted CLI/API/runtime behavior is ready; repository merge readiness remains owned by the separate workstream-close gate.
