# QA Run Report — 2026-08-20 — PR 440 CodeRabbit remediation

- **Scope:** User-visible review fixes for dialogs, onboarding, extension installation, settings, sandbox, vault, task editing, and error disclosure in PR #440
- **Cadence tier:** targeted
- **Build:** d3291d24-dirty · **Environment:** isolated local daemon `127.0.0.1:56354`, production Web proxy, agent-browser, desktop / local loopback / en-US
- **Started:** 2026-08-20T23:13:20Z · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | fresh isolated runtime home | desktop / local loopback / en-US | CH-untested-019-19-lea |
| Dora | fresh isolated runtime home | desktop / local loopback / en-US | CH-untested-041-administer-runtime-settings-dora |
| Bruno | fresh isolated runtime home | desktop / local loopback / en-US | CH-extension-marketplace-skill-canary, CH-task-template-draft |

## Flows in Scope

- `J-19` — finish first-run setup without retaining an operator-home workspace draft.
- `J-extension-distribution` — understand and submit an extension source from the marketplace.
- `J-administer-runtime-settings` — reach settings and write-only boundaries without leaking raw failures.
- `J-complete-task-tree` — keep the authored task scope visible while editing a draft.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-untested-019-19-lea | J-19 / RT-004 | Lea | Feature Tour | Pass | | |
| 2 | CH-extension-marketplace-skill-canary | J-extension-distribution / ET-web-extension-union-install | Bruno | Feature Tour | Pass | | |
| 3 | CH-untested-041-administer-runtime-settings-dora | J-administer-runtime-settings / MS-web-modal-help-tips | Dora | Accessibility Tour | Pass | | |
| 4 | CH-task-template-draft | J-complete-task-tree / TA-task-template-preserves-draft | Bruno | Feature Tour | Pass | | |
| 5 | CH-untested-062-manage-sandbox-profiles-dora | J-manage-sandbox-profiles / MS-web-sandbox-profile-advanced | Dora | Error Tour | Pass | | |
| 6 | CH-untested-061-keep-secrets-contained-dora | J-keep-secrets-contained / MS-038 | Dora | Error Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### First-run setup — Lea

- **Ran:** 2026-08-20T23:16:00Z → 2026-08-20T23:18:00Z (box respected: yes)
- **Findings:** The live model catalog selected Codex, Workspace did not retain the operator home as a selected workspace, and Skip completed setup in Global scope.
- **Bugs filed/updated:** None.
- **Scenarios settled:** RT-004 → pass.
- **Paper cuts:** None.
- **Surprises:** The isolated runtime had a complete native model catalog without provider-backed sessions.
- **Suggested next charter:** Replay a stale persisted workspace identity after the onboarding draft schema changes again.

### Extension, task, Sandbox, Vault, and Settings — Bruno and Dora

- **Ran:** 2026-08-20T23:18:00Z → 2026-08-20T23:30:00Z (box respected: yes)
- **Findings:** The extension repository HelpTip exposed the safe HTTPS/version guidance; task template switching preserved authored fields and kept the Global scope statement; Sandbox Advanced and Vault boundaries remained visible; all dialogs stayed usable and no browser errors were recorded.
- **Bugs filed/updated:** None.
- **Scenarios settled:** ET-web-extension-union-install, MS-web-modal-help-tips, TA-task-template-preserves-draft, MS-web-sandbox-profile-advanced, and MS-038 → pass for the changed review contracts.
- **Paper cuts:** None.
- **Surprises:** None.
- **Suggested next charter:** Exercise the sanitized error states through a real failing upstream once an injectable production-parity fault boundary exists.

## What Was Fixed

Review remediation was completed before this QA run; any in-session defect will be recorded here.

## Paper Cuts

None observed.

## Runtime Errors Observed

No product errors appeared in the browser console. The daemon reported expected missing optional provider CLIs and credentials in its status payload; none affected the walked flows.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Evidence

- Behavioral log: `/Users/pedronauck/dev/qa-labs/compozy-pr-440-coderabbit-20260820-230901-470715-lab/qa-artifacts/qa/journey-log.jsonl`
- Extension HelpTip: `/Users/pedronauck/dev/qa-labs/compozy-pr-440-coderabbit-20260820-230901-470715-lab/qa-artifacts/qa/screenshots/extension-git-help-tip.png`
- Preserved task draft: `/Users/pedronauck/dev/qa-labs/compozy-pr-440-coderabbit-20260820-230901-470715-lab/qa-artifacts/qa/screenshots/task-draft-approval.png`
- Settings: `/Users/pedronauck/dev/qa-labs/compozy-pr-440-coderabbit-20260820-230901-470715-lab/qa-artifacts/qa/screenshots/settings-general.png`
- Teardown: `/Users/pedronauck/dev/qa-labs/compozy-pr-440-coderabbit-20260820-230901-470715-lab/qa-artifacts/qa/teardown.json` (`clean: true`)

## Learnings

- The fresh runtime is enough to prove review-copy and interaction boundaries without introducing test-only data into the live daemon.
- Parser and network-failure disclosure paths remain stronger in their canonical suites than in browser QA that would have to forge an upstream response.

## Final Status

Pending.
