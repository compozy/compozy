# QA Run Report — 2026-08-15 — PR 410 review evidence

- **Scope:** PR #410 review remediation for workspace overview, worktree lifecycle, and shortcut surfaces
- **Cadence tier:** targeted
- **Build:** ac37a89f-dirty · **Environment:** isolated local daemon `127.0.0.1:59271`, Web `http://localhost:3000`, agent-browser, desktop / wifi-fast / en-US
- **Started:** 2026-08-15T06:07:48Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | local isolated workspace | desktop / wifi-fast / en-US | CH-workspaces-command-switcher, CH-add-workspace-from-root |
| Bruno | local isolated workspace | desktop / local loopback / en-US | CH-worktree-destructive-recovery |

## Flows in Scope

- `J-operate-workspace-context` — switch the active workspace and worktree without losing identity.
- `J-worktree-management` — create, adopt, inspect, and remove linked worktrees safely.
- `J-add-workspace-by-browsing` — adjacent canary for returning from an empty workspace catalog.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-workspaces-command-switcher | J-operate-workspace-context / RT-workspace-overview-command-tab | Ada | Feature Tour | Pass | | |
| 2 | CH-workspaces-command-switcher | J-worktree-management / RT-worktree-web-create-adopt | Ada | Feature Tour | Pass | | |
| 3 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-reconcile-branch-safety | Bruno | Interrupt Tour | Pass | | |
| 4 | CH-add-workspace-from-root | J-add-workspace-by-browsing / adjacent canary | Ada | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-workspaces-command-switcher — Ada

- **Ran:** 2026-08-15T06:02:00Z → 2026-08-15T06:10:00Z (box respected: yes)
- **Findings:** No functional finding. The shortcut opened the overlay; Home, End, arrows, Escape layering, pointer menus, empty-state Enter/Space, and focus return reached the intended live target.
- **Bugs filed/updated:** None.
- **Scenarios settled:** RT-workspace-overview-command-tab → pass; RT-worktree-web-create-adopt → pass.
- **Paper cuts:** None.
- **Surprises:** A large discovered catalog remained keyboard reachable while keeping New worktree available.
- **Suggested next charter:** Re-run with a live catalog deletion while focus is on the removed middle row.

### CH-worktree-destructive-recovery — Bruno

- **Ran:** 2026-08-15T06:05:00Z → 2026-08-15T06:10:00Z (targeted leg; box respected: yes)
- **Findings:** Normal removal deleted the checkout, retained the removed catalog row, and preserved refs/heads/qa-pr410-20260815 at its recorded head.
- **Bugs filed/updated:** None.
- **Scenarios settled:** RT-worktree-reconcile-branch-safety → pass.
- **Paper cuts:** None.
- **Surprises:** The selected deleted worktree fell back to the workspace scope while the overlay stayed usable.
- **Suggested next charter:** Repeat the full interrupt tour for an exit-operation release candidate.

### CH-add-workspace-from-root — Ada

- **Ran:** 2026-08-15T06:09:00Z → 2026-08-15T06:11:00Z (box respected: yes)
- **Findings:** From the empty overview, Space and Enter opened Add workspace; the visible folder browser reached the checkout and registration survived reload.
- **Bugs filed/updated:** None.
- **Scenarios settled:** Adjacent canary → pass.
- **Paper cuts:** None.
- **Surprises:** None.
- **Suggested next charter:** Preserve this canary for future empty-catalog changes.

## What Was Fixed

Review remediation was completed before this QA run; no in-session production fix has been required.

## Paper Cuts

None observed.

## Runtime Errors Observed

No product runtime errors. The broad package command `go test -race ./internal/store/globaldb` exhausted its 10-minute suite timeout under heavy parallel migration/task tests; the three affected canonical suites passed separately in 24.378 seconds.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- One-marker selection remained truthful across create, adopt, delete, refresh, and Global scope.
- The failure story makes the retryable terminal state visible without freezing the implementation in a snapshot test.

## Public Screenshot Evidence

- [Current workspace and worktree](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-410/workspaces-current-worktree.png)
- [Worktree actions](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-410/worktree-actions-menu.png)
- [Create worktree](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-410/new-worktree-dialog.png)
- [Duplicate worktree refusal](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-410/create-existing-branch-validation.png)
- [Retryable terminal creation failure](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-410/create-terminal-failure-story.png)
- [Adopt worktree](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-410/adopt-worktree-confirmation.png)
- [Remove while preserving branch](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-410/remove-worktree-confirmation.png)
- [Empty overview](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-410/workspaces-empty-state.png)
- [Shortcut reference](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-410/keyboard-shortcuts-workspaces.png)
- [Shortcut settings](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-410/settings-workspaces-shortcut.png)

Teardown evidence: `/Users/pedronauck/dev/qa-labs/compozy-pr-410-review-evidence-20260815-055849-475301-lab/qa-artifacts/qa/teardown.json` (`clean: true`).

## Compozy Impact Audit

- Native tools: worktree removal resolves canonical IDs before mutation; native-tool and transport parity suites cover ID/name forwarding and removal semantics.
- Extensibility and hooks: worktree CLI, HTTP, UDS, SSE catalog, native tools, official skill, and config shortcut allowlist were checked; no tool ID or hook-event rename.
- Workspace data isolation: workspace and worktree IDs remain scoped through CLI/HTTP/UDS/core/store/Web catalog reads; this run uses a unique `COMPOZY_HOME`, port, socket, and browser session.
- Official Compozy skill: `skills/compozy/references/worktrees.md` documents normal-removal branch preservation.

## Final Status

- **Exit gate (full automated suite):** CI-owned per operator instruction; `make gate-full` was intentionally not run locally. Exception evidence: `/Users/pedronauck/dev/qa-labs/compozy-pr-410-review-evidence-20260815-055849-475301-lab/qa-artifacts/qa/ci-owned-final-make-verify.log`.
- **Local verification:** focused Go lint passed with zero issues; 7 focused Web files / 117 tests passed; focused Go race suites passed; Web/UI/Site lint and typecheck passed; codegen-check passed; React Doctor 100/100; `git diff --check` passed.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0.
- **Coverage:** 4/4 matrix rows passed across Web, CLI, and HTTP API; adjacent Add workspace canary passed.
- **Verdict:** PASS — targeted local QA is ready; the PR-wide full gate remains owned by CI.
