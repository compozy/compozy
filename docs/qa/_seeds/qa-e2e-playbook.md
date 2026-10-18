# Compozy QA / E2E Playbook

> Living document for manual and future automated end-to-end validation.
> Seeded from real daemon + web + `agent-browser` execution on 2026-04-14.

## 1. Purpose

This document defines the QA / E2E flows that must be exercised regularly for Compozy.
It is intended to serve three purposes:

1. A repeatable manual checklist for future QA rounds.
2. A source of truth for what "working" means at the product level.
3. A direct blueprint for future browser/CLI E2E automation.

The emphasis is not "green tests at any cost". The emphasis is:

- real runtime behavior
- real daemon startup and shutdown
- real web interactions
- real persistence and recovery
- real mutation flows
- real validation of failure modes

When a check fails, the default assumption must be "product bug until disproven", not "test flake".

---

## 2. Core QA Principles

### 2.1 Non-negotiable rules

- Always run against a real built binary, not only mocked frontend hooks.
- Always use an isolated `COMPOZY_HOME` for QA so local user state is not polluted.
- Always cross-check critical UI mutations with CLI or persisted backend state.
- Always rerun the full repo gate with `make verify` before calling a round complete.
- Always treat raw `5xx`, stuck spinners, and silent no-op mutations as failures.
- Always capture enough evidence to prove persistence, not just optimistic UI updates.

### 2.2 Evidence standard

A flow is only considered validated when all of the following are true:

- The action can be performed in the UI or public CLI surface.
- The backend accepts it without hidden errors.
- The resulting state is visible in at least one independent read path.
- The state survives the relevant lifecycle boundary:
  - refresh
  - deep link
  - daemon restart
  - session resume

### 2.3 Permanent regression guardrails

These regressions are now known product risks and must always be checked:

- `Network` with `network.enabled = false` must render an explicit disabled state, not raw downstream `503` errors.
- `Automation` create/edit flows must never submit `retry.strategy = "none"` with non-zero `max_retries` or non-empty `base_delay`.
- Browser-visible mutations must not silently fail while leaving the editor open with no actionable error.

---

## 3. Canonical Execution Profiles

Use these profiles as named fixtures for future rounds and future E2E harnesses.

### Profile A: Baseline default config

Purpose:

- Validate first-run and safe default behavior.

Key expectations:

- `network.enabled = false`
- app loads
- onboarding works
- `Network` page shows disabled state
- no hard failures on empty states

### Profile D: Multi-workspace profile

Purpose:

- Validate workspace scoping, active workspace switching, and add-dir agent visibility.

Key expectations:

- at least one repo workspace
- at least one extra registered workspace
- at least one agent only visible in the secondary workspace

---

## 4. Mandatory Fresh Gate Before and After QA

Every substantial QA round should start and end with:

```bash
make verify
```

When the round includes web changes or frontend bug fixes, also keep these available for tighter loops:

```bash
make bun-lint
make web-typecheck
make web-test
```

Failure policy:

- If `make verify` fails before QA starts, the round is already red.
- If `make verify` fails after a bug fix, the round is still red even if the browser flow looks fixed.

---

## 5. Suite Matrix

| Suite ID | Area                                                   | Priority | Cadence                          | Must be automatable later? |
| -------- | ------------------------------------------------------ | -------- | -------------------------------- | -------------------------- |
| `BASE`   | Repo gate + build contract                             | P0       | Every round                      | Yes                        |
| `BOOT`   | Install, daemon lifecycle, onboarding                  | P0       | Every round                      | Yes                        |
| `WS`     | Workspaces, switching, navigation                      | P0       | Every round                      | Yes                        |
| `SES`    | Sessions, prompting, transcript, resume                | P0       | Every round                      | Yes                        |
| `AUTO`   | Automation jobs/triggers and runs                      | P0       | Every round                      | Yes                        |
| `BRIDGE` | Provider discovery and delivery                        | P1       | Every round when provider exists | Yes                        |
| `NET`    | Disabled mode, enabled mode, channels, peers, messages | P0       | Every round                      | Yes                        |
| `ROB`    | Restart, reload, reconnect, recovery                   | P0       | Every round                      | Yes                        |
| `ERR`    | Empty/loading/error states                             | P1       | Every round                      | Yes                        |
| `CONS`   | CLI/UI consistency audit                               | P0       | Every round                      | Yes                        |

---

## 6. Detailed Test Cases

Each case below includes the behavior to exercise, the minimum assertions, and the edge cases that must stay covered.

### `E2E-BASE-001` Fresh repository verification

Goal:

- Prove the repo is in a runnable state before product-level QA starts.

Steps:

1. Run `make verify`.

Assertions:

- lint passes
- typecheck passes
- unit/integration suite passes
- build completes
- package-boundary checks pass

Always fail if:

- any target is skipped silently
- warnings are ignored
- QA proceeds without a fresh pass

---

### `E2E-BOOT-001` Isolated runtime bootstrap and daemon start

Goal:

- Prove Compozy can bootstrap and run in a clean isolated home.

Steps:

1. Create a temporary `COMPOZY_HOME`.
2. Run `./bin/compozy install`.
3. Start the daemon.
4. Check `./bin/compozy status`.

Assertions:

- daemon reports `running`
- HTTP endpoint is present
- socket is present
- install writes valid config and home layout

Edge coverage:

- first boot from empty state
- no dependency on an already-populated `~/.compozy`

---

### `E2E-BOOT-002` First workspace onboarding in the web app

Goal:

- Prove a clean browser can onboard a repo workspace from the real web flow.

Steps:

1. Open the app in a real browser.
2. Complete onboarding for the target repo.

Assertions:

- workspace appears in the UI
- the app exits onboarding into the main shell
- sidebar and main pages load without reload loops

Always fail if:

- onboarding completes visually but the workspace is not persisted
- the shell loads but critical API calls fail

---

### `E2E-WS-001` Active workspace switching and scope isolation

Goal:

- Prove the active workspace actually changes backend-visible state, not just UI highlight.

Steps:

1. Register at least two workspaces.
2. Make one workspace expose additional agent dirs unavailable in the other.
3. Switch active workspace in the app.
4. Open a flow that depends on workspace agent resolution, such as channel creation.

Assertions:

- agent lists change with workspace selection
- workspace-specific entities appear only where expected
- sidebar session grouping matches the active workspace filter

Edge coverage:

- workspace with `additional_dirs`
- repo workspace vs secondary temp workspace

---

### `E2E-WS-002` Refresh, deep link, and sidebar continuity

Goal:

- Prove internal routes survive direct navigation and reload.

Steps:

1. Open internal routes directly:
   - `/automation`
   - `/network`
   - `/session/<id>`
2. Refresh each page.
3. Use browser back/forward where useful.

Assertions:

- route loads without redirect corruption
- selected entity detail panel rehydrates correctly
- sidebar remains usable after refresh

Always fail if:

- a deep link renders an empty shell without loading the target entity
- refresh destroys state that should have been server-backed

---

### `E2E-SES-001` Create a session and send a first prompt

Goal:

- Prove the main chat flow works end-to-end from the web shell.

Steps:

1. Create a new session from the sidebar.
2. Send a simple deterministic prompt.

Assertions:

- session appears in the sidebar
- transcript shows user and agent messages
- message persists on refresh
- `session status` or transcript endpoint confirms the same session state

Edge coverage:

- initial empty transcript
- first message in a newly created session

---

### `E2E-SES-002` Concurrent session prompts

Goal:

- Prove multiple sessions can be active and respond independently.

Steps:

1. Create at least two sessions in different workspaces or with different agents.
2. Send deterministic prompts to both without waiting for the first to finish.

Assertions:

- both sessions remain addressable
- both eventually produce the expected response
- one slow session does not starve the other

Always fail if:

- the second prompt cancels or corrupts the first
- events are attached to the wrong session

---

### `E2E-SES-003` Stop and resume a session

Goal:

- Prove explicit interruption and resume work on a live session.

Steps:

1. Start a real session.
2. Stop it.
3. Confirm stopped state.
4. Resume it.
5. Send another deterministic prompt.

Assertions:

- stop reason is recorded correctly
- resumed session returns to `active`
- post-resume prompt produces the expected response
- transcript preserves both pre-stop and post-resume history

---

### `E2E-SES-004` Session persistence after daemon restart

Goal:

- Prove session metadata and transcripts remain available after daemon shutdown.

Steps:

1. Create or reuse persisted sessions.
2. Stop the daemon.
3. Start it again.
4. Reopen the session from the sidebar or deep link.
5. Resume at least one stopped session.

Assertions:

- stopped sessions still appear in the browser shell
- deep link to `/session/<id>` still loads transcript
- resumed session works after restart

Important nuance:

- `compozy session list` without `--all` only shows active sessions.
- `--all` includes stopped sessions but still excludes archived sessions by default.
- Use `compozy session list --all --include-archived` when checking the full persisted catalog after shutdown.

---

### `E2E-AUTO-001` Create an automation job from the UI

Goal:

- Prove the browser create flow persists a valid job.

Steps:

1. Open `Automation`.
2. Switch to the correct workspace scope if needed.
3. Create a new job from the UI.

Assertions:

- dialog closes on success
- job appears in the list and detail panel
- CLI returns the same job
- payload is valid for the backend contract

Permanent regression assertions:

- when `retry.strategy = "none"`, the payload must use:
  - `max_retries = 0`
  - `base_delay = ""`
- the create flow must not return `400 Bad Request`

Always fail if:

- button is enabled but clicking is a silent no-op
- the editor stays open after a successful mutation with no error shown

---

### `E2E-AUTO-002` Create an automation trigger from the UI

Goal:

- Prove the trigger creation flow works with a valid contract payload.

Steps:

1. Switch to the `TRIGGERS` tab.
2. Create a new trigger with a real event.

Assertions:

- dialog closes on success
- trigger appears in the list and detail panel
- CLI returns the same trigger
- retry payload normalization matches backend requirements

Edge coverage:

- `webhook` event with webhook-only fields
- non-webhook event with webhook fields absent

---

### `E2E-AUTO-003` Trigger a job and verify run history

Goal:

- Prove automation execution is real, not only CRUD.

Steps:

1. Trigger a real job via CLI or UI.
2. Wait for a run record to appear.
3. Inspect run history.
4. Inspect spawned session where applicable.

Assertions:

- a run ID is created
- status transitions are visible
- spawned session exists when expected
- run history persists after refresh

---

### `E2E-AUTO-004` Automation validation errors

Goal:

- Prove invalid inputs are rejected cleanly and specifically.

Minimum cases:

- invalid trigger configuration:
  - `event = ext.test.qa`
  - non-empty `endpoint_slug`
- invalid retry payload:
  - `strategy = none`
  - non-zero `max_retries`
  - non-empty `base_delay`

Assertions:

- backend returns validation error, not generic `500`
- UI surfaces a useful error state where applicable
- no partial entity is persisted

---

### `E2E-ROB-001` Daemon restart while the UI is in use

Goal:

- Prove the app survives server unavailability and eventual recovery.

Steps:

1. Keep the browser on an internal page.
2. Stop the daemon.
3. Start it again.
4. Reload or revisit the route.

Assertions:

- persisted entities remain visible after recovery
- deep links still work
- sessions can be resumed and used again

Important checks:

- no stale UI-only state masking backend loss
- no route becomes permanently broken after recovery

---

### `E2E-ROB-002` Reload, reconnect, and browser navigation behavior

Goal:

- Prove the web shell does not depend on a single long-lived in-memory path.

Steps:

1. Navigate between pages:
   - `Automation`
   - `Bridges`
   - `Network`
   - `Session`
2. Refresh each page.
3. Use browser back and forward.

Assertions:

- page-specific detail state rehydrates
- selected item remains valid or falls back sanely
- no duplicated toasts or broken listeners appear after navigation

---

### `E2E-ERR-001` Empty, loading, and error-state sweep

Goal:

- Prove each feature has intentional UX for non-happy paths.

Minimum coverage:

- `Automation`: empty list, loading, mutation error
- `Bridges`: no provider / no bridge / delivery validation error
- `Network`: disabled, empty, and enabled populated states
- `Sessions`: empty sidebar and persisted stopped sessions

Assertions:

- empty states are descriptive and actionable
- loading states are visible and finite
- error states are specific and not raw transport dumps where avoidable

---

### `E2E-CONS-001` CLI / UI consistency audit

Goal:

- Prove the browser is showing server truth.

Steps:

1. Create or mutate entities through the UI.
2. Read the same entities through the CLI.
3. Create or mutate entities through the CLI.
4. Verify the browser reflects them.

Must-cover entities:

- sessions
- jobs
- triggers
- bridges
- channels
- peers

Assertions:

- IDs match
- scope/workspace bindings match
- state transitions match
- persisted timestamps and counts are plausible

---

## 7. Always-Check Assertions By Surface

### Sessions

- Session IDs are stable across reads.
- Transcript survives refresh.
- Stop/resume transitions are visible in status and transcript.
- Restart does not destroy persisted stopped sessions.

### Workspaces

- Active workspace changes data, not only selection chrome.
- Workspace-scoped entities do not leak into unrelated scopes.
- Additional-dir agents appear only where they should.

### Automation

- Create/edit/delete are not silent no-ops.
- Retry contract is always valid.
- Triggered runs create observable backend state.

### Resilience

- Restart semantics are explicit:
  - active sessions may stop on shutdown
  - persisted sessions must remain inspectable
  - resumed sessions must become usable again

---

## 8. Future Automation Strategy

This document should eventually map to executable suites, not only manual rounds.

### Recommended split

1. `smoke-e2e`
   - daemon boot
   - onboarding
   - create session
   - send prompt
   - open major routes

2. `stateful-e2e`
   - multiple workspaces
   - stop/resume
   - restart recovery
   - deep links

3. `network-e2e`
   - disabled mode
   - enabled mode
   - channel creation
   - peers
   - message send

4. `automation-e2e`
   - create job
   - create trigger
   - trigger run
   - validation failures

5. `bridges-e2e`
   - provider discovery
   - create bridge
   - delivery test paths

### Recommended execution model

- Use an isolated `COMPOZY_HOME` fixture per suite or per worker.
- Use public CLI to seed data only where the UI is not the target under test.
- Use browser automation only for flows whose primary contract is UI behavior.
- Always validate critical mutations through a second read path.

### Recommended artifacts to capture

- `daemon status -o json`
- CLI entity listings in JSON
- browser snapshots or screenshots at key checkpoints
- HAR for failed mutation debugging
- route-specific console/errors when a UI mutation fails

---

## 9. Round Exit Criteria

A QA round is only complete when all of the following are true:

- the planned P0 suites were exercised
- every discovered regression was either fixed or explicitly documented as a blocker
- the fix, if any, was verified in the real browser/runtime path
- `make verify` passed after the last code change
- no result is based only on mocked confidence

If any of those are missing, the round is incomplete.

---

## 10. Suggested First Automation Backlog

If we convert this playbook into executable E2E tests, start with these in order:

2. `E2E-AUTO-001` create job with valid `retry.none`
3. `E2E-AUTO-002` create trigger with valid `retry.none`
4. `E2E-SES-001` create session and send prompt
5. `E2E-SES-004` restart + resume + transcript recovery
6. `E2E-WS-001` workspace switching and agent visibility

These cover the highest-signal product contracts and the two regressions already found in real usage.
