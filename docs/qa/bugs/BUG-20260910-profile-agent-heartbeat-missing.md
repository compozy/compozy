# BUG-20260910-profile-agent-heartbeat-missing: A newly created agent cannot report its Heartbeat status

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-11 Inspect agent policy and session correlation
- **Scenarios:** RT-024
- **Found:** 2026-09-10 · **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction

- **Charter:** CH-014, adjacent policy diagnosis after the inspector walk.
- **Environment:** Isolated daemon and default profile, workspace field-notes, CLI/HTTP, actual Codex Luna xhigh sessions.

1. Create `field-writer` with `agent create --workspace field-notes`, selecting Codex Luna xhigh.
2. Start a session with that agent and complete a native terminal read.
3. Read `agent info field-writer --workspace field-notes` and `agent heartbeat status field-writer --workspace field-notes`.
4. Read the session inspect endpoint with wake events requested.

**Expected:** The same existing agent resolves on both surfaces; an absent Heartbeat policy is a valid empty policy with configuration correlation.
**Actual:** Agent info succeeds and identifies the `project_profile` layer. Heartbeat status returns `agent_not_found`, looking under `.compozy/agents/field-writer` although the created definition is under `.compozy/profiles/default/agents/field-writer`. Session inspect silently omits policy/config correlation through its existing missing-agent handling.

## Evidence

Under `docs/qa/evidence/2026-09-10-qa-execution-unblock/`:

- `writer-create.json`, `heartbeat-agent-info.json`: successful creation and current definition.
- `heartbeat-status-current.json`: failing public status read.
- `ledger-enabled-settled-http.json`, `ledger-enabled-prompt.json`: successful real provider run and selected model.
- `ledger-enabled-active-inspect.json`: missing correlation fields.

## Diagnosis and local correction

The filesystem contains the profile-owned AGENT.md. A read-only diagnostic query found no `field-writer` agent resource record. The daemon publisher explicitly discovers global/workspace agents, while its profile discovery helper publishes only skills. The authored-context fallback resolves an unprofiled workspace path. Repair must preserve profile visibility and source precedence across catalog and authored-context surfaces; simply returning an empty policy would hide the missing source.

The core authored-context resolver now uses the same profile-aware workspace resolution as agent info. Agent reads and mutations use the request's resolved profile; session status/inspect and wake-event enrichment use the session's immutable owner profile. This restores filesystem discovery without making catalog publication a prerequisite or changing source precedence. Missing-agent behavior for truly deleted definitions is retained.

The canonical core authored-context suite reproduced two 404 policy reads and omitted session correlation before the change. The added regression loads real default/marketing agent files with different policy digests and verifies session-owner selection despite a different browsing profile. Its health reader is an explicit unit I/O boundary; provider execution remains exclusively in the QA lab. Full suite and live re-walk are in progress.

### Cross-surface impact

- HTTP/UDS and CLI: existing Soul/Heartbeat and session inspection routes retain their payloads and now resolve the selected/owning profile. No new verbs, IDs, or schema.
- Web: inspector correlation uses the backend session owner; no additional UI control or copy is required.
- Workspace data: no persisted shape, migration, or file move. Homonymous definitions remain scoped to their profile.
- Native tools: independent native Heartbeat resolution still needs an actual public-surface check; no native acceptance is inferred from core coverage.
- Extensibility/hooks/config: no changed hook/config contract. Package-owned catalog policy resolution is unchanged and is not covered by this filesystem repair.
- Official skill (`skills/compozy/`) and Web/Docs: command contracts remain valid; scenario and bug documentation record corrected source resolution. No skill interface changes.

The prior disabled-ledger correction is independent.

## Verified local fix

Commit `4f5ae5298` contains the production correction and canonical regressions. `make gate` passed all affected lanes (`profile-context-make-gate-retry.txt`); the live RT-024 Web/HTTP/UDS retest and restart parity are recorded in `profile-patch-parity-summary.json` and `profile-patch-memory-web.png`. No push or CI claim.

## Native adjacent canary follow-up

After the core/API fix, actual Cursor session `sess-7f562c591bdbbc98` called `compozy__agent_heartbeat_status` exactly once for the same field-writer/workspace and received `tool_backend_failed`. Its transcript and completed prompt are retained as `cursor-policy-prompt.json` / `cursor-policy-events.json`; the session was stopped through CLI. This reopens the finding for the independent native entry point; the RT-024 HTTP/UDS/Web branch remains verified.

The native handler discarded `Scope.ProfileID` and resolved an unprofiled workspace. The local follow-up carries the caller profile into the existing native profile-aware workspace resolver for status and wake. No tool ID/input, authorization policy, or persisted shape changes. Regression coverage was added to the existing native-tools suite using two real profile-owned agent/policy files and the real status service with a store I/O stub. The first test invocation had a helper import/namespace build error; it was corrected, not treated as a behavioral red result. The real managed-agent failure is the before-fix proof. Re-walk and gate are pending.

Native follow-up verified: commit `f32f50a9c`; focused native tests and `make gate` passed. Fresh managed Cursor Grok 4.6 High Fast session `sess-967598658cb4e67e` called the same tool once and returned `missing`, correctly representing the absent HEARTBEAT.md policy. `cursor-policy-retake-prompt.json`, `cursor-policy-retake-events.json`, `native-heartbeat-retake-runtime.json`, and `cursor-policy-retake-stop.json` retain the retest and lifecycle proof. The session was verified stopped. No push or external CI claim.
