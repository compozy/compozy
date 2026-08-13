# QA Run Report — 2026-08-13 — PR 368 CodeRabbit remediation

- **Scope:** Targeted re-walk of user-visible Global/workspace boundary fixes in PR 368
- **Cadence tier:** targeted
- **Build:** `7e5d6b8` (latest re-walk; initial pass used `a97e07f`) · **Environment:** isolated daemon and Web lab from the PR head
- **Started:** 2026-08-13T05:18:21Z · **Status:** pass with explicit human verification blocks

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-global-scope-regression, CH-marketplace-scope-isolation |
| Dora | Power User | desktop / wifi-fast / en-US | CH-create-destination-regression |
| Lea | New User | laptop / wifi-fast / en-US | CH-onboarding-global-skip |
| Cora | Casual User | laptop / wifi-fast / en-US | CH-home-preload-recovery |

## Flows in Scope

- `J-operate-desktop-shell` — shell entry points preserve one truthful scope (`../journeys/J-operate-desktop-shell.md`)
- `J-31` — destination-bound creation survives abandonment and reload (`../journeys/J-31-steward-agent-definition.md`)
- `J-19` — first-run setup completes without racing workspace changes (`../journeys/J-19-onboarding-default-model.md`)
- `J-mcp-authorize-repair` — MCP state and credentials remain scoped (`../journeys/J-mcp-authorize-repair.md`)
- `J-operate-home-dashboard` — Home remains reachable and truthful through query failure (`../journeys/J-operate-home-dashboard.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-global-scope-regression | J-operate-desktop-shell / MS-web-menubar-global-scope-toggle; ET-web-command-palette-shortcuts; MS-web-session-deeplink-global-confirm | Bruno | Feature Tour | Blocked (needs human verify) | | |
| 2 | CH-create-destination-regression | J-31 / MS-web-create-destination-derived | Dora | Back-Button Tour | Blocked (needs human verify) | | |
| 3 | CH-onboarding-global-skip | J-19 / RT-onboarding-skip-to-global | Lea | Interrupt Tour | Pass | | |
| 4 | CH-marketplace-scope-isolation | J-mcp-authorize-repair / ET-web-marketplace-mcp-authorize-installed | Bruno | Multi-Tab Tour | Blocked (needs human verify) | | |
| 5 | CH-home-preload-recovery | J-operate-home-dashboard / RT-home-dashboard-zones | Cora | Network Tour | Fixed | BUG-20260813-retry-leaves-blank-route | `a97e07f` |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-global-scope-regression — Bruno

- **Ran:** 2026-08-13T05:28:00Z → 2026-08-13T05:29:00Z (box respected: yes)
- **Findings:** Globe and palette transitions preserved project `tmp`; refresh retained the project and the project menu excluded the operator-home registration.
- **Bugs filed/updated:** None.
- **Scenarios settled:** MS-web-menubar-global-scope-toggle → pass; ET-web-command-palette-shortcuts → pass; MS-web-session-deeplink-global-confirm → blocked-verify because the isolated lab had no Global session id.
- **Paper cuts:** None.
- **Surprises:** Switching scope intentionally closed open windows, so the restored project was verified by reopening Home.
- **Suggested next charter:** Create one Global session and walk both confirm and decline deep-link branches.

### CH-create-destination-regression — Dora

- **Ran:** 2026-08-13T05:53:00Z → 2026-08-13T05:56:00Z (box respected: yes)
- **Findings:** Abandoning a project Knowledge draft and reopening after switching Global produced a clean form with the Global destination statement. The MCP install dialog independently reported Global and retained no prior form state.
- **Bugs filed/updated:** None.
- **Scenarios settled:** MS-web-create-destination-derived → blocked-verify; this pass covered Knowledge and MCP abandonment but did not submit every listed resource type.
- **Paper cuts:** None.
- **Surprises:** None.
- **Suggested next charter:** Submit one resource per create surface and compare each public list after scope changes.

### CH-onboarding-global-skip — Lea

- **Ran:** 2026-08-13T05:25:00Z → 2026-08-13T05:28:00Z (box respected: yes)
- **Findings:** None. Skip remained available with zero project folders; add and removal completed before setup could finish.
- **Bugs filed/updated:** None.
- **Scenarios settled:** RT-onboarding-skip-to-global → pass.
- **Paper cuts:** None.
- **Surprises:** The directory browser resolves local additions too quickly to hold a visible pending state under wifi-fast; the canonical hook regression covers the in-flight guard.
- **Suggested next charter:** Re-run the same mission with an intentionally slow remote filesystem.

### CH-home-preload-recovery — Cora

- **Ran:** 2026-08-13T05:29:00Z → 2026-08-13T05:50:00Z (box respected: yes)
- **Findings:** Retry initially removed the workspace error and left Home blank after the daemon returned (Trust-Damage). Commit `a97e07f` kept recovery visible and made app-shell preloads opportunistic.
- **Bugs filed/updated:** BUG-20260813-retry-leaves-blank-route → fixed.
- **Scenarios settled:** RT-home-dashboard-zones → pass after a fresh interruption/recovery walk.
- **Paper cuts:** None beyond the functional recovery failure.
- **Surprises:** A fresh browser session reaches the same route, isolating the defect to the Retry transition.
- **Suggested next charter:** Repeat with a throttled workspace response instead of a full daemon interruption.

### CH-marketplace-scope-isolation — Bruno

- **Ran:** 2026-08-13T05:56:00Z → 2026-08-13T05:58:00Z (box respected: yes)
- **Findings:** The Global Airtable install opened clean and named Global as its destination. No project draft credential crossed into it.
- **Bugs filed/updated:** None.
- **Scenarios settled:** ET-web-marketplace-mcp-authorize-installed → blocked-verify; a real OAuth consent and token-presence check needs a human credential owner.
- **Paper cuts:** None.
- **Surprises:** None.
- **Suggested next charter:** Complete OAuth using a disposable human-owned account and inspect the installed projection in both scopes.

## What Was Fixed

The remediation commits `916ef01e` and `8314e5da` address CodeRabbit and React Compiler findings. QA then found `BUG-20260813-retry-leaves-blank-route`; `a97e07f` fixes the root Retry transition and app-shell preload failure boundary.

GitHub E2E later exposed a second production regression: Global correctly had no active project, but mounted runtime apps also lost the hidden operator-home daemon binding. Commit `7e5d6b8` moves Agents, Sessions, Automation suggestions, Loops, Network, restored-window preloads, and session fallbacks to `runtimeWorkspaceId`. Explicit Global/project filters in Marketplace, Tasks, and Knowledge remain on `activeWorkspaceId`.

## Post-CI Regression Re-walk

- `agent-categories.spec.ts`: 1 passed.
- `agents.spec.ts`: 7 passed; Global agent creation omitted `workspace`.
- Automation and Network focused journeys: 4 passed.
- Loop focused journeys: 5 passed.
- Session hardening, onboarding, and clarification journeys: 9 passed.
- Full Web unit suite: 572 files and 4,635 tests passed.
- Evidence: `/Users/pedronauck/dev/qa-labs/compozy-pr-368-global-runtime-binding-20260813-091321-434791-lab/qa-artifacts/qa/verification-report.md`.

## Paper Cuts

- `WorkspaceApiError: Failed to fetch workspaces: 502` was expected while the daemon was intentionally stopped. The failed Retry was fixed and re-walked.

## Runtime Errors Observed

- The Home activity SSE connection reported an error while the daemon was intentionally stopped. It recovered with the daemon and did not persist after Retry.

## Human Verifications Needed

- [ ] Complete real OAuth consent and confirm token presence for the installed MCP server (row #4).
- [ ] Create a Global session and walk both deep-link confirmation branches (row #1).
- [ ] Submit every remaining creation surface and confirm its public list destination (row #2).

## Decisions for a Human

None.

## Learnings

- Taxonomy plan: functional, error/recovery, and continuity are covered by the five sessions. Locale is deliberately skipped because this change adds no localized copy. Mobile is skipped because the affected shell and creation surfaces are desktop-only in this cycle.

## Final Status

**Verdict: PASS** for the exercised PR behavior. One QA-discovered P1 regression was fixed and re-walked. The three blocked legs above require human-owned credentials or broader fixture creation and do not hide a known failure.

The later GitHub E2E runtime-binding regression was also fixed and re-walked. Global agent creation and runtime-bound Global views now pass against the production daemon and production Web bundle.

The final CodeRabbit pass found two edge cases. Commit `da635f5` prevents a Global loop run from displaying an unrelated project workspace name and treats an empty session runtime workspace ID as unavailable. The focused controller suites passed 9/9, React Doctor found no issues, and the daemon-served loop suite passed 5/5 on the final build.

- **Final make verify evidence:** `/Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/logs/final-make-verify.log`
- **Teardown evidence:** `/Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/teardown.json`
