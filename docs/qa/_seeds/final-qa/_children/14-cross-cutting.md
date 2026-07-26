---
name: 14-cross-cutting
description: QA child report — cross-cutting integration scenarios that span ≥3 modules. These scenarios fail only when the seams between modules are broken, even when each module's own QA passes.
type: qa-child
module: cross-cutting
sources:
  - internal/daemon
  - internal/session
  - internal/task
  - internal/scheduler
  - internal/hooks
  - internal/memory
  - internal/memory/consolidation
  - internal/skills
  - internal/tools
  - internal/extension
  - internal/automation
  - internal/network
  - internal/coordinator
  - internal/bridges
  - internal/api/contract
  - internal/api/core
  - internal/api/httpapi
  - internal/api/udsapi
  - internal/observe
  - internal/transcript
  - internal/sse
  - internal/settings
  - internal/store
  - internal/workspace
  - web/src
  - packages/site
composes:
  - 01-daemon-boot (DB-04, DB-05, DB-10, DB-13, DB-14, DB-15)
  - 02-config-settings (cfg-04, cfg-05, cfg-06, cfg-08, cfg-10, cfg-14)
  - 03-acp-sessions (ACP-04, ACP-05, ACP-08, ACP-12, ACP-15, ACP-19)
  - 04-autonomy-kernel (AUT-01, AUT-03, AUT-04, AUT-05, AUT-09, AUT-12, AUT-15, AUT-16, AUT-18)
  - 05-memory-soul (MEM-01, MEM-04, MEM-09, MEM-10, MEM-11, MEM-19)
  - 06-skills-capabilities (SKL-01, SKL-03, SKL-09, SKL-11, SKL-13)
  - 07-tools-sandbox (TOL-01, TOL-03, TOL-08, TOL-10, TOL-14, TOL-15)
  - 08-extensions-bridges (EXT-01, EXT-02, EXT-03, EXT-06, EXT-11, EXT-14, EXT-15, EXT-16)
  - 09-automation-cron (CRN-01, CRN-04, CRN-05, CRN-14, CRN-18)
  - 10-network-identity (NET-01, NET-02, NET-05, NET-12, NET-19)
  - 11-api-cli-parity (API-02, API-05, API-07, API-08, API-09, API-13, API-17)
  - 15-observability (OBS-01, OBS-02, OBS-03, OBS-05, OBS-06, OBS-18)
---

# Cross-Cutting Integration — Final QA Plan

## 1. Module Surface

This child does not own a single Go package. It owns the **seams between packages** — the contracts that two or more modules must agree on for a real operator journey to succeed end-to-end. Every scenario in this file requires ≥3 modules to behave correctly, and would still fail even when every individual module's QA child reports "Worked".

The cross-cutting surface is implicit in:

- **Composition root** (`internal/daemon/daemon.go`, `internal/daemon/boot.go`) — only place that imports every subsystem; bootCleanup LIFO unwinding is the only place that proves "every Start has a Shutdown".
- **Authoritative-primitive exclusivity** (L-005, `docs/_memory/lessons/L-005-authoritative-primitive-exclusivity.md`) — `task.Service.ClaimNextRun` is the only claim authority; the mechanical scheduler and manual prompts compose **through** it, never around it.
- **Detached execution lifetime** (`internal/CLAUDE.md:33-35`, L-001) — prompts, network channel sends, automation jobs, and bridge dispatches all share the `context.WithoutCancel(ctx)` rule; a regression in any one of them silently breaks the others.
- **Codegen co-ship contract** (`internal/api/contract/`, `openapi/agh.json`, `web/src/generated/agh-openapi.d.ts`, `agh-contract-codegen-coship` skill) — backend contract edits cross three repos in one commit; partial regen anywhere breaks the boundary.
- **Canonical correlation keys** (`internal/CLAUDE.md:49`) — every domain operation emits `workspace_id, session_id, parent_session_id, root_session_id, agent_name, task_id, run_id, claim_token_hash, lease_until, workflow_id, coordinator_session_id, scheduler_reason, hook_event, hook_name, spawn_depth, actor_kind, actor_id, release_reason`. A scenario that touches three subsystems must show all the correlation keys those subsystems should populate, not just the keys for the originating subsystem.
- **Hook taxonomy + dispatch** (`internal/hooks/`, `internal/CLAUDE.md:24-26`) — typed dispatch at the call site that owns the state transition; never tail event tables. A hook that fires from the wrong call site is invisible to single-module QA but breaks composition.
- **Truthful UI** (`internal/CLAUDE.md` truthful-UI rule, SD-007) — the web UI is incomplete if it renders a control whose backend is not shipped; the web build must reject backend-ahead-of-frontend AND frontend-ahead-of-backend states equally.
- **Extension surface symmetry** (`internal/extension/`, `internal/CLAUDE.md:24-26`) — installing an extension must atomically appear on CLI verbs, HTTP routes, UDS verbs, web UI, hooks, and memory tracking; an extension that exposes a CLI verb but no UDS verb is a silent partial-surface bug.
- **Network ↔ identity ↔ task** (`internal/network/`, `internal/agentidentity/`, `internal/task/`) — `claim_token_hash` traverses the wire as a hash; the raw `claim_token` MUST never cross the network. This is the cross-cutting test that single-module QA on `internal/task` cannot prove.

The endpoints, CLI verbs, and persistence layers used in each scenario below are owned by other children and cited by their scenario IDs. The combinations are unique to this child.

## 2. Existing Test Coverage Map

There is **no single Go test target** for cross-cutting integration. The closest existing coverage is fragmented across:

- `internal/daemon/daemon_*_integration_test.go` — full daemon harness against `internal/acp/acpmock` (mock ACP). These are wiring tests; they prove the daemon assembles every subsystem but never prove a real ACP-speaking subagent moves through ≥3 subsystems.
- `internal/coordinator/coordinator_test.go` — coordinator wiring; uses mock task service.
- `internal/hooks/dispatch_integration_test.go` — hook ordering across subsystems but with stub services.
- `internal/api/httpapi/httpapi_integration_test.go` — HTTP↔SSE↔EventStore round-trip; does not cross into network or extension.
- `internal/extension/manager_integration_test.go` — extension lifecycle but limited to extension-internal surfaces.
- `internal/memory/consolidation/runtime.go` — Time→Sessions→Lock cascade; consolidation tests do not assert that the cascade is driven by a cron-fired session that came in via automation.

**Suspicious / coverage-shaped gaps already documented**:

- DB module's coverage map (`01-daemon-boot.md` §2.9) flags every `internal/daemon/daemon_*_integration_test.go` as wiring-only (`acpmock`, not real Claude Code). Cross-cutting QA must close this for at least every scenario marked `live: true` below.
- `04-autonomy-kernel.md` AUT-15 already calls out the cron→coordinator→child overlap as a cross-module scenario; this child re-anchors it from the "cross-cutting" perspective and adds the full chain.
- L-007 (`docs/_memory/lessons/L-007-e2e-follows-runtime-contract.md`) — E2E mocks/matchers MUST ship with contract changes. The codegen co-ship scenario (XCT-15 below) is the QA gate that proves L-007 is enforced.
- L-001 (`docs/_memory/lessons/L-001-detached-prompt-lifetime.md`) — detached lifetime is the cross-cutting invariant; XCT-01 and XCT-08 below force this through three subsystems each.
- L-005 (`docs/_memory/lessons/L-005-authoritative-primitive-exclusivity.md`) — `task.Service.ClaimNextRun` exclusivity. XCT-04 and XCT-05 below require it to compose with cron, scheduler, and ACP without a peer claim path appearing.

## 3. Coverage Gaps

| Gap | Claim AGH makes | Test that would make the claim load-bearing |
| --- | --- | --- |
| **Config hot-apply across daemon restart preserves resumable session** | `internal/CLAUDE.md` (composition-root discipline + detached lifetime); cfg-05 promises restart-required flag is honored. | XCT-01: change a restart-required key, daemon orderly restart, real Claude Code session recoverable, transcript continuous, SSE post-restart replay starts from `after_seq` of last pre-shutdown durable append. |
| **Bundle install atomically wires skill + tool + hook + audit + SSE + web** | `internal/extension/manager.go`, `internal/skills/registry.go`, hooks declarations, `internal/observe`, web `useExtensions` hook. | XCT-02: bundle install at runtime; real Claude Code uses the bundled skill on next prompt, hook denies a tool dispatch, audit row recorded, SSE event reaches the live web UI subscriber within 2 s. |
| **Soul + memory + hook + tool deny chain runs in correct order on a single call** | `internal/soul`, `internal/memory/recall.go`, `internal/hooks/ordering.go`, `internal/tools/policy.go`. | XCT-03: agent SOUL.md narrows tool capability set, recalled memory triggers a hook narrowing, denied tool path produces stable typed error and ledger row. |
| **Cron fires → child session → real LLM → events → web UI → consolidation gate** | `internal/automation`, `internal/scheduler`, `internal/session`, `internal/observe`, `internal/memory/consolidation`. | XCT-04: a 1-minute cron runs five times; touched_sessions counter reaches 5 and the Sessions gate fires the consolidation runtime exactly once; real Claude Code child receives the cron-fired prompt and writes a marker; SSE shows lineage `parent_session_id → root_session_id`. |
| **Coordinator startup → claim → spawn → MCP tool → ledger row → CLI parity** | `internal/coordinator`, `internal/task`, `internal/acp`, `internal/mcp`, `internal/observe`, CLI parity. | XCT-05: fresh AGH_HOME; coordinator boots, claims first task_run, spawns subagent via ACP, subagent calls hosted MCP tool; ledger captures redacted tool call; `agh sessions list -o json` reflects exactly the same state HTTP returns. |
| **Two-instance network: claim_token_hash crosses, raw `claim_token` never does** | `internal/network` rejects raw claim_token in metadata (`internal/CLAUDE.md:55`); NET-05 covers ingress, NET-19 covers audit. | XCT-06: instance A delegates a sub-task to B's exposed agent via channel; B completes; lineage `root_session_id` correlates both transcripts; `claim_token_hash` appears in B's audit row but the literal regex `\bagh_claim_[A-Za-z0-9]+\b` matches zero bytes across all of A's and B's logs/SSE/audit. |
| **Extension uninstall is atomic across CLI/HTTP/UDS/web/memory/hooks** | `internal/extension/manager.go`, no partial-surface completions rule (`internal/CLAUDE.md:27`). | XCT-07: install ext-foo; observe CLI verb, HTTP route `/api/ext/foo`, UDS verb, web capability tile, memory hook firing on every invocation; uninstall; observe all four surfaces atomically removed and memory hook stops firing. |
| **SSE reconnect across daemon restart with `Last-Event-ID` / `after_seq` durability** | OBS-05 anchors single-module; this scenario adds bridge inbound + tool dispatch in flight at restart. | XCT-08: client SSE-subscribes; bridge inbound (Slack) starts a real Claude Code prompt that runs a tool; SIGTERM mid-tool; client retries with the last seq; new daemon replays durably-appended events without duplicates. |
| **Bridge inbound → real agent → tool call → bridge outbound: full actor correlation** | `internal/bridges`, `internal/session`, `internal/tools`, observability. | XCT-09: real Slack message creates a session via `internal/bridges`, real Claude Code answers with a tool-driven response; outbound matches the same Slack thread; events show `actor_kind=bridge, actor_id=slack:<channel>` for both inbound and outbound. |
| **Multi-workspace memory + settings + sessions + web all isolate by `workspace_id`** | cfg-08 (config), MEM-11/MEM-12 (memory), web workspace switcher. | XCT-10: workspaces W_A and W_B; one Claude Code session in each running concurrently; memory writes never cross; settings overlay differs; web switcher renders both; `workspace_id` correlation key present on every event of each session. |
| **Failure cascade is contained: extension panic does not kill peers** | `internal/extension/manager.go` lifecycle, EXT-15 single-module, daemon supervision. | XCT-11: install ext-X with a panicking host-API call; daemon catches; ext-X marked unhealthy with backoff; all other agents/sessions unaffected; web shows badge; `agh extension list -o json` reflects; restart-policy attempt within window. |
| **Greenfield zero-legacy invariant on a fresh AGH_HOME** | `CLAUDE.md` greenfield rule; SD-002. | XCT-12: brand-new AGH_HOME; assert no migration paths fire for "old state"; assert grep-search for the names of deleted features (e.g. `recipe`, `playbook`, deprecated CLI verbs) produces zero hits in the running binary's CLI output, OpenAPI spec, web bundle, and docs site. |
| **Lifecycle hook chain across three layers in one session** | `internal/hooks/ordering.go`, `internal/skills`, `internal/extension/manager.go`, EXT-14, MEM-09. | XCT-13: bundle B installs skills + hooks at three layers (bundled, marketplace, workspace); session start fires hooks in hierarchy precedence then alphabetical order; one hook errors → fail-open, subsequent hooks run; ledger contains every timing. |
| **Truthful UI gate: web is rejected when ahead of daemon** | SD-007 truthful-UI; `internal/CLAUDE.md` truthful-UI rule. | XCT-14: deliberately add a control to `web/` for an endpoint that does not exist in `internal/api/contract/`; QA gate must reject the build. |
| **Codegen co-ship: contract-edit-then-codegen-check is the only legitimate path** | `agh-contract-codegen-coship` skill; L-007. | XCT-15: edit `internal/api/contract/`; `make codegen-check` fails; run `make codegen`; both `openapi/agh.json` and `web/src/generated/agh-openapi.d.ts` updated; `make codegen-check` now passes; web TypeScript build green. |
| **SD-005 audit: every module child has at least one real-LLM scenario** | SD-005 (real-scenario QA); CLAUDE.md release rule. | XCT-16: a top-level audit script greps every `_children/*.md` for at least one fenced `qa-scenario` with `live: true` AND `provider: claude-code`/`openclaw`/`hermes`; missing → release-ready verdict denied. |

## 4. Real-LLM / Real-Agent Scenarios

Each scenario is one fenced block. Numbered XCT-01..XCT-16. Live runs use real subagents. Every scenario has a `composes:` field listing the single-module scenario IDs whose surfaces are exercised in combination — this is the cross-cutting traceability hook.

```markdown
### XCT-01 — Config restart-required key → orderly restart → real Claude Code session resumes; transcript continuous

```yaml qa-scenario
id: xct-01-config-restart-session-resume-transcript-continuity
title: Restart-required config change preserves an active real-LLM session and its transcript across daemon restart
theme: cross-cutting
composes:
  - cfg-05-restart-required-flag
  - cfg-04-hot-apply-disabled-skills
  - DB-04-crash-recovery-task-run
  - DB-10-binary-upgrade
  - ACP-08
  - ACP-15
  - API-05-sse-reconnect-after-seq
  - OBS-05-sse-replay-across-restart
coverage:
  primary:
    - cross.config-restart.session-resume
    - cross.session.transcript-continuity
  secondary:
    - settings.classify.restart-required
    - sse.replay.after-seq
    - daemon.shutdown.graceful
live: true
provider: claude-code
modules: [config, settings, daemon, session, transcript, sse, observe]
```

```yaml qa-flow
preconditions:
  - Fresh lab (`agh-qa-bootstrap`); real Claude Code reachable; AGH_HOME, ports, PROVIDER_HOME isolated.
  - AGH_WEB_API_PROXY_TARGET exported when web is exercised.
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/xct01 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Acknowledge with a single line. Then wait." --detach -o json
  - run: curl -N "http://127.0.0.1:$AGH_HTTP_PORT/api/sessions/$SID/events?stream=1" >sse.pre.log &
  - set: SSE_PID=$!
  - sleep: 3
  - run: agh config set acp.subprocess.shutdown_grace_seconds 8 -o json | tee cfgset.json   # restart-required key per cfg-05
  - run: jq -r '.classification' cfgset.json   # MUST be "restart-required"
  - run: LAST_SEQ=$(tail -n 30 sse.pre.log | grep -oE 'id: [0-9]+' | tail -1 | awk '{print $2}')
  - run: kill -TERM $(jq -r '.pid' "$LAB_HOME/daemon.json")
  - sleep: 12   # > defaultShutdownTimeout
  - run: kill -0 $SSE_PID 2>/dev/null || echo "client disconnected"
  - run: agh daemon start && sleep 5
  - run: curl -N -H "Last-Event-ID: $LAST_SEQ" "http://127.0.0.1:$AGH_HTTP_PORT/api/sessions/$SID/events?stream=1" >sse.post.log &
  - sleep: 3
  - run: agh sessions resume $SID -o json | tee resume.json
  - run: agh sessions prompt $SID --message "Confirm previous acknowledgement and stop." -o json
  - run: agh sessions transcript $SID -o json | tee transcript.json
  - run: jq 'length' transcript.json
  - run: jq '.[].sequence' transcript.json | awk 'NR>1 && $1<=p {print "OUT-OF-ORDER"; exit 1} {p=$1}'
  - run: grep -E '\bagh_claim_[A-Za-z0-9]+\b' "$LAB_HOME/logs/agh.log" sse.pre.log sse.post.log; echo $?
expected_behavior:
  - cfgset.json reports `classification: "restart-required"` (settings classify per `internal/settings/classify.go:80-129`).
  - SIGTERM triggers orderly shutdown (DB-05): every ACP subprocess gone, `daemon.json` removed, `daemon.lock` zeroed.
  - On boot, `bootSessionRepair` runs and the session is recoverable; `agh sessions resume` succeeds.
  - sse.post.log resumes from `after_seq=LAST_SEQ` (no duplicates, no skips); per `internal/api/httpapi/middleware.go:45-47` `Last-Event-ID` is honored.
  - transcript.json contains pre-restart user/assistant pair AND post-restart pair; sequences strictly monotonic.
  - Greedy claim_token grep returns exit code 1 (no match) across every captured artifact.
evidence_to_capture:
  - sess.json, cfgset.json, resume.json, transcript.json.
  - sse.pre.log, sse.post.log.
  - $LAB_HOME/logs/agh.log between the two daemon runs.
  - $LAB_HOME/daemon.json before SIGTERM and after second start.
failure_signatures:
  - cfgset.json reports `classification: "hot-apply"` for a key the table marks restart-required.
  - sse.post.log starts from sequence 1 (replay missed the seek).
  - transcript.json missing the pre-restart prompt.
  - Any literal `agh_claim_…` in any captured artifact.
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### XCT-02 — Bundle install → bundled skill → real agent uses skill → hook denies a tool → SSE event lands in web UI

```yaml qa-scenario
id: xct-02-bundle-skill-hook-deny-sse-web
title: A bundle installation atomically wires a skill that a real Claude Code agent uses; a hook denies one of the skill's tool calls; the deny is observable on SSE/web/audit
theme: cross-cutting
composes:
  - EXT-01
  - EXT-02
  - SKL-01
  - SKL-11
  - TOL-14
  - AUT-03
  - OBS-01
  - API-17
coverage:
  primary:
    - cross.bundle.skill-tool-hook-audit-sse-web
  secondary:
    - extension.lifecycle.activation
    - hooks.dispatch.deny
    - observe.sse.realtime
live: true
provider: claude-code
modules: [extension, skills, hooks, tools, observe, api, web]
```

```yaml qa-flow
preconditions:
  - Lab home; real Claude Code; web dev server pointed at `AGH_WEB_API_PROXY_TARGET` from bootstrap.env.
  - Bundle fixture under ./fixtures/xct02-bundle/ contains: `manifest.toml` declaring one skill (`xct-skill`), one toolset (`xct__example`), one `pre_tool_use` hook for tool name `xct__example__write` that returns `{decision:"deny", reason:"qa-test"}`.
steps:
  - run: agh daemon start && sleep 5
  - run: agh extension install ./fixtures/xct02-bundle -o json | tee install.json
  - run: jq '.status, .surfaces[]' install.json   # MUST list skills, tools, hooks each as registered
  - run: agh skill list -o json | jq '.[] | select(.name=="xct-skill")' | tee skill.json
  - run: agh tool list -o json | jq '.[] | select(.id=="xct__example__write")' | tee tool.json
  - run: curl -N "http://127.0.0.1:$AGH_HTTP_PORT/api/observe/events?stream=1&type=tool_call,hook_decision" >sse.log &
  - run: agh sessions start --agent claude-code --workspace ./fixtures/xct02-workspace -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Use the xct-skill capability to write 'hello' to NOTES.md (tool: xct__example__write)." -o json | tee prompt.json
  - sleep: 5
  - run: jq '.[] | select(.event_type=="hook_decision" and .session_id=="'$SID'")' < sse.log | tee hook_decisions.json
  - run: jq '.[] | select(.event_type=="tool_call" and .session_id=="'$SID'")' < sse.log | tee tool_calls.json
  - run: agh sessions transcript $SID -o json | tee transcript.json
  - run: agh observe events --session $SID --type hook_decision -o json | tee audit_hook.json
  - run: agh observe events --session $SID --type tool_call -o json | tee audit_tool.json
  - run: curl -s "http://127.0.0.1:$AGH_HTTP_PORT/api/extensions" | jq '.[] | select(.name=="xct02-bundle")' | tee ui_extensions.json
expected_behavior:
  - install.json shows `surfaces: [skills, tools, hooks]` registered atomically (rollback-on-partial-failure tested in EXT-02).
  - hook_decisions.json contains exactly one entry: `{decision:"deny", reason:"qa-test", hook_event:"pre_tool_use", hook_name:"xct02-bundle/deny-write"}` with required correlation keys (`session_id`, `task_id`, `claim_token_hash` — never raw `claim_token`).
  - tool_calls.json shows the call attempt with status `denied` (not `started`); per AUT-03, deny short-circuits dispatch BEFORE provider-side spawn.
  - transcript.json includes a denial system message but no successful tool result.
  - audit_hook.json and audit_tool.json (UDS) match the SSE entries (HTTP + UDS parity per BaseHandlers, API-04).
  - ui_extensions.json shows the bundle as installed; web subscriber received the SSE entries within 2 s.
evidence_to_capture:
  - install.json, skill.json, tool.json, sess.json, prompt.json.
  - sse.log, hook_decisions.json, tool_calls.json.
  - transcript.json, audit_hook.json, audit_tool.json, ui_extensions.json.
failure_signatures:
  - install.json shows partial surfaces (skill registered but hook missing).
  - hook fires AFTER tool_call event (hook ordering bug).
  - hook_decisions.json missing `claim_token_hash`.
  - SSE event `tool_call` shows `status: started` even though hook denied.
  - Web `/api/extensions` does NOT list the new bundle.
cleanup:
  - agh sessions stop $SID && agh extension uninstall xct02-bundle && agh daemon stop
```
```

```markdown
### XCT-03 — Agent soul + memory recall + hook narrowing + tool deny on unsafe path

```yaml qa-scenario
id: xct-03-soul-memory-hook-tool-deny-unsafe-path
title: SOUL.md asserts safety policy; memory recall surfaces it; pre_tool_use hook narrows write capability; agent attempts an unsafe path; tool dispatch denies it.
theme: cross-cutting
composes:
  - MEM-01
  - MEM-19
  - AUT-04
  - TOL-03
  - TOL-15
  - SKL-13
coverage:
  primary:
    - cross.soul-memory-hook-tool.deny-unsafe
  secondary:
    - memory.recall.workspace-scope
    - hooks.dispatch.narrow
    - tools.policy.path-allowlist
live: true
provider: claude-code
modules: [soul, memory, hooks, tools, sandbox]
```

```yaml qa-flow
preconditions:
  - Lab home with workspace ./fixtures/xct03-workspace/ containing AGENT.md and SOUL.md.
  - SOUL.md states: "I refuse to write outside the workspace root. I will refuse traversal."
  - Memory seeded: a feedback entry "User got bitten by a `..` path; reject any write that resolves outside the workspace."
  - Hook fixture: workspace-level pre_tool_use hook that adds a metadata field `policy.write_allowlist=["./"]` (narrow, not deny).
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/xct03-workspace -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh agent context $SID -o json | tee context.json
  - run: jq '.soul.body' context.json | grep -i 'refuse'
  - run: jq '.memory.recall[].body' context.json | grep -i 'traversal'
  - run: agh sessions prompt $SID --message "Write 'pwn' to ../escape.txt." -o json | tee prompt.json
  - sleep: 4
  - run: agh observe events --session $SID --type hook_decision -o json | tee hooks.json
  - run: agh observe events --session $SID --type tool_call -o json | tee tools.json
  - run: agh sessions transcript $SID -o json | tee transcript.json
  - run: ls ../escape.txt 2>&1 | grep -i 'no such file'
expected_behavior:
  - context.json shows soul + memory recall entries; assembler ordered as MEM-19 (soul block, memory index, recall block).
  - hooks.json: one `decision: "narrow"` entry with `metadata.policy.write_allowlist=["./"]`.
  - tools.json: write call attempted; final `status: "denied"` with `reason` referencing path policy (per TOL-03 sandbox + TOL-15 narrowing).
  - transcript.json shows agent acknowledging refusal in natural language consistent with SOUL.md.
  - ../escape.txt does not exist.
evidence_to_capture:
  - sess.json, context.json, prompt.json, hooks.json, tools.json, transcript.json.
failure_signatures:
  - hooks.json missing the narrow entry → hook didn't fire at the right call site.
  - tools.json shows `status: "completed"` for the unsafe path → policy not enforced.
  - escape.txt exists.
  - context.json missing the recall body → memory index didn't reach assembler.
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### XCT-04 — Cron one-minute → child session → real Claude Code → canonical events → web stream → consolidation Sessions gate increments

```yaml qa-scenario
id: xct-04-cron-child-session-realllm-events-web-consolidation
title: Cron fires every minute; each fire creates a child session that runs a real Claude Code prompt; events stream to web; Sessions consolidation gate increments touched_sessions and fires on the 5th
theme: cross-cutting
composes:
  - CRN-01
  - CRN-14
  - CRN-18
  - ACP-04
  - OBS-01
  - OBS-02
  - OBS-18
  - MEM-04
  - API-17
coverage:
  primary:
    - cross.cron.child.real-llm.events.consolidation-sessions-gate
  secondary:
    - automation.scheduler.one-minute
    - memory.consolidation.sessions-gate
    - observe.sse.realtime
live: true
provider: claude-code
modules: [automation, scheduler, session, acp, observe, memory.consolidation, api]
```

```yaml qa-flow
preconditions:
  - Lab home; real Claude Code; consolidation enabled; consolidation Sessions gate threshold = 5 (default per `internal/CLAUDE.md` memory consolidation gates).
steps:
  - run: agh daemon start && sleep 5
  - run: agh automation cron add --schedule "*/1 * * * *" --prompt "Append a line 'tick-{{run_index}}' to LOG.md and stop." --workspace ./fixtures/xct04-workspace --delivery isolated -o json | tee cron.json
  - set: CID=$(jq -r '.id' cron.json)
  - run: curl -N "http://127.0.0.1:$AGH_HTTP_PORT/api/observe/events?stream=1" >sse.log &
  - sleep: 330   # 5.5 minutes — five fires
  - run: agh observe events --type session_started --since "$(date -d '6 minutes ago' --utc -Iseconds)" -o json | jq 'length' | tee sessions.count
  - run: agh observe events --type cron_fired --since "$(date -d '6 minutes ago' --utc -Iseconds)" -o json | jq 'length' | tee cron.count
  - run: agh memory consolidation status -o json | tee consolidation.json
  - run: jq '.sessions_gate.touched_count, .sessions_gate.last_fired_at' consolidation.json
  - run: agh sessions list --since "$(date -d '6 minutes ago' --utc -Iseconds)" -o json | tee child_sessions.json
  - run: jq '.[].lineage | {root_session_id, parent_session_id, agent_name, actor_kind, actor_id}' child_sessions.json
  - run: cat ./fixtures/xct04-workspace/LOG.md
expected_behavior:
  - cron.count >= 5 (five fires).
  - sessions.count >= 5 (every fire spawned a fresh child session per CRN-14).
  - consolidation.json: `sessions_gate.touched_count >= 5`, `sessions_gate.last_fired_at` is a timestamp inside the run window (gate fired exactly once on the 5th touch per MEM-04).
  - child_sessions.json: every entry has `actor_kind=automation, actor_id=cron:<CID>`, identical `root_session_id` for child + cron-driven; `parent_session_id` set if delivery is "child" (or null for `isolated`).
  - LOG.md contains five lines `tick-1` ... `tick-5`.
  - sse.log contains `cron_fired`, `session_started`, `tool_call`, `agent_message`, `session_stopped` events for every fire, with required correlation keys per OBS-02.
evidence_to_capture:
  - cron.json, sse.log, sessions.count, cron.count, consolidation.json, child_sessions.json, LOG.md.
failure_signatures:
  - touched_count < 5 → counter not incrementing on session.stopped.
  - touched_count == 5 but `last_fired_at` is null → gate didn't fire.
  - LOG.md missing entries → child session didn't actually call Claude Code.
  - sse.log missing any of the canonical events → coverage matrix gap.
cleanup:
  - agh automation cron remove $CID && rm ./fixtures/xct04-workspace/LOG.md && agh daemon stop
```
```

```markdown
### XCT-05 — Coordinator → claim → ACP spawn → MCP tool → ledger redaction → CLI parity

```yaml qa-scenario
id: xct-05-coordinator-claim-acp-mcp-ledger-cli
title: Coordinator boots on a fresh AGH_HOME, claims first task_run, spawns subagent via ACP, subagent calls hosted MCP tool with secret in args; ledger redacts secret; `agh sessions list -o json` matches HTTP exactly
theme: cross-cutting
composes:
  - AUT-09
  - AUT-01
  - AUT-12
  - ACP-04
  - TOL-08
  - TOL-10
  - OBS-06
  - API-04
coverage:
  primary:
    - cross.coordinator.spawn.mcp.redaction.cli-parity
  secondary:
    - autonomy.task-runs.single-queue
    - tools.mcp.sidecar
    - observe.redaction.bearer
live: true
provider: claude-code
modules: [coordinator, task, acp, mcp, tools, observe, cli, api]
```

```yaml qa-flow
preconditions:
  - Fresh `$LAB_HOME`. Coordinator agent enabled in config.
  - Hosted MCP fixture under ./fixtures/xct05-mcp/ exposing tool `secret_lookup(key:string, token:string)`.
  - One queued task_run inserted as part of bootstrap (or written via `agh task add` after start).
steps:
  - run: agh daemon start && sleep 7   # coordinator bootstrap window
  - run: jq -r '.coordinator.session_id' "$LAB_HOME/daemon.json" | tee coord_sid.txt
  - run: agh task add --workspace ./fixtures/xct05-workspace --prompt "Look up the secret using token=BEARERSECRET123 then summarize." -o json | tee task.json
  - set: TID=$(jq -r '.id' task.json)
  - sleep: 8
  - run: agh task get $TID -o json | tee taskrun.json
  - run: jq '.claim_token_hash, .claim_token' taskrun.json
  - run: agh sessions list -o json > sessions.cli.json
  - run: curl -s "http://127.0.0.1:$AGH_HTTP_PORT/api/sessions" > sessions.http.json
  - run: jq -S . sessions.cli.json > a.json && jq -S . sessions.http.json > b.json && diff a.json b.json
  - run: agh observe events --task $TID --type tool_call -o json | tee tool.events.json
  - run: jq '.[] | {tool_id, args, result}' tool.events.json | grep -E '(BEARERSECRET123|REDACTED)'
  - run: grep -E '(BEARERSECRET123|\bagh_claim_[A-Za-z0-9]+\b)' "$LAB_HOME/logs/agh.log"; echo $?
  - run: agh task list --filter "session=$(jq -r '.[0].id' sessions.cli.json)" -o json | tee tasks.json
expected_behavior:
  - Coordinator session id present in `daemon.json`; coord_sid.txt non-empty.
  - taskrun.json: `claim_token_hash` is sha256 hex; raw `claim_token` field absent (per AUT-09 + AUT-16).
  - sessions.cli.json byte-identical to sessions.http.json (CLI ↔ HTTP parity, API-04).
  - tool.events.json: every `args.token` is `[REDACTED]` per `internal/diagnostics/redact.go` Bearer matcher (TOL-08).
  - grep returns exit 1 (no BEARERSECRET, no claim_token literal) across all logs.
  - tasks.json shows the run completed (`state: succeeded`) with `claim_token_hash`.
evidence_to_capture:
  - coord_sid.txt, task.json, taskrun.json, sessions.cli.json, sessions.http.json, tool.events.json, tasks.json, agh.log excerpt.
failure_signatures:
  - taskrun.json contains a `claim_token` key.
  - sessions.cli.json differs from sessions.http.json.
  - tool.events.json shows `BEARERSECRET123` literally.
  - agh.log contains `BEARERSECRET123` or `agh_claim_…`.
cleanup:
  - agh task cancel $TID && agh daemon stop
```
```

```markdown
### XCT-06 — Local/Live Network: one bounded wake preserves lineage and raw claim tokens never cross

```yaml qa-scenario
id: xct-06-live-network-claim-hash-correlation
title: A Local control stays disconnected while an explicitly Live session receives one addressed wake with task-run lineage and no raw claim token disclosure
theme: cross-cutting
composes:
  - NET-01
  - NET-02
  - NET-05
  - NET-12
  - NET-19
  - AUT-12
  - AUT-16
  - API-17
  - OBS-02
coverage:
  primary:
    - cross.network.delegate.claim-hash.lineage
  secondary:
    - network.participation.live
    - autonomy.lineage.correlation
    - security.claim-token.network-rejection
live: true
provider: claude-code
modules: [network, agentidentity, task, session, transcript, observe]
```

```yaml qa-flow
preconditions:
  - One fresh isolated lab from `agh-qa-bootstrap`; real Claude Code reachable under the manifest provider policy.
  - Network available and channel `agh-qa-channel` created.
steps:
  - Create one Local control session with `agh session new --network local -o json` and save its resolved snapshot.
  - Create one Live session with `agh session new --network live --network-channel-strategy named --network-channel agh-qa-channel -o json`.
  - Send one addressed `say` with nonce `OK-XCT06` to the Live session through the structured Network surface.
  - Wait for the bounded wake to settle, then read the conversation, wake task run, session transcript, Network usage, and correlated events.
  - Read the Local control's prompt/environment/tool projection and prove it received no Network state or wake.
  - Scan all captured bodies, events, transcripts, and logs for raw `agh_claim_*` values.
expected_behavior:
  - The Local snapshot is immutable Local and has zero Network prompt, tools, wake, and usage delta.
  - The Live transcript acknowledges `OK-XCT06` exactly once and shares the wake task run's `root_session_id`/`run_id` lineage.
  - Conversation and recipient disposition are durable before the wake; usage is actual or `usage_unavailable`.
  - Structured evidence may expose `claim_token_hash`; no response, event, transcript, or log exposes raw `claim_token`.
evidence_to_capture:
  - local-session.json, live-session.json, conversation.json, wake-run.json, transcript.json, usage.json, events.json, forbidden-needles.txt.
failure_signatures:
  - Local receives any Network affordance or activation.
  - The nonce is absent or prompts more than once.
  - Conversation, wake, transcript, and usage lineage disagree.
  - Any raw `agh_claim_*` value appears.
cleanup:
  - Run the bootstrap manifest teardown command and retain `teardown.json` with `clean: true`.
```
```

```markdown
### XCT-07 — Extension install lights up CLI + HTTP + UDS + web; uninstall removes all four atomically; memory hook fires on each invocation

```yaml qa-scenario
id: xct-07-extension-cli-http-uds-web-symmetry
title: ext-foo install registers a CLI verb, HTTP route, UDS verb, web capability tile; memory hook fires on each invocation; uninstall removes all four surfaces atomically and stops the hook
theme: cross-cutting
composes:
  - EXT-01
  - EXT-03
  - EXT-11
  - EXT-14
  - MEM-09
  - API-04
  - API-13
coverage:
  primary:
    - cross.extension.symmetry.install-uninstall
  secondary:
    - extension.lifecycle.activation-rollback
    - hooks.dispatch.session-post-create
live: true
provider: claude-code
modules: [extension, cli, api.httpapi, api.udsapi, web, memory, hooks]
```

```yaml qa-flow
preconditions:
  - Lab home; real Claude Code; web running with proxy target from bootstrap.env.
  - Bundle ext-foo declares: CLI verb `agh ext-foo lookup`, HTTP route `/api/ext/foo/lookup`, UDS verb `ext.foo.lookup`, host-API hook `on_session_invoked` writing a memory entry tagged `provenance:ext-foo`.
steps:
  - run: agh daemon start && sleep 5
  - run: agh extension install ./fixtures/xct07-ext-foo -o json | tee install.json
  - run: agh ext-foo lookup --query smoke -o json | tee cli.json
  - run: curl -s "http://127.0.0.1:$AGH_HTTP_PORT/api/ext/foo/lookup?query=smoke" | tee http.json
  - run: agh ipc invoke ext.foo.lookup --query smoke -o json | tee uds.json
  - run: curl -s "http://127.0.0.1:$AGH_HTTP_PORT/api/extensions/capabilities" | jq '.[] | select(.name=="ext-foo")' | tee web_capability.json
  - run: jq -S . cli.json > cli_n.json && jq -S . http.json > http_n.json && jq -S . uds.json > uds_n.json && diff cli_n.json http_n.json && diff http_n.json uds_n.json
  - run: agh sessions start --agent claude-code --workspace ./fixtures/xct07 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Run ext-foo lookup with query=hello and stop." -o json
  - sleep: 4
  - run: agh memory list --tag provenance:ext-foo -o json | tee mem.json
  - run: jq 'length >= 1' mem.json   # at least one entry per invocation
  - run: agh extension uninstall ext-foo -o json | tee uninstall.json
  - run: ! agh ext-foo lookup --query smoke 2>&1   # CLI verb gone
  - run: curl -s -o /dev/null -w "%{http_code}\n" "http://127.0.0.1:$AGH_HTTP_PORT/api/ext/foo/lookup"   # 404
  - run: ! agh ipc invoke ext.foo.lookup --query smoke 2>&1   # UDS verb gone
  - run: curl -s "http://127.0.0.1:$AGH_HTTP_PORT/api/extensions/capabilities" | jq '.[] | select(.name=="ext-foo")' | tee web_after.json
  - run: agh sessions prompt $SID --message "Try ext-foo lookup again." -o json
  - sleep: 3
  - run: agh memory list --tag provenance:ext-foo --since "$(date -d '1 minute ago' --utc -Iseconds)" -o json | tee mem_after.json
expected_behavior:
  - cli.json, http.json, uds.json normalise to identical content (API-04 + API-13).
  - web_capability.json non-empty before uninstall.
  - mem.json shows at least one entry per invocation; hook fired in hierarchy precedence then alphabetical order (per `internal/CLAUDE.md` lifecycle hooks rule).
  - uninstall.json `status: "uninstalled"`; CLI returns "unknown command", HTTP returns 404, UDS returns `ErrUnknownVerb`, web_after.json empty.
  - mem_after.json empty for the post-uninstall invocation (no new entry).
evidence_to_capture:
  - install.json, cli.json, http.json, uds.json, web_capability.json, sess.json, mem.json.
  - uninstall.json, web_after.json, mem_after.json.
failure_signatures:
  - Any of CLI/HTTP/UDS/web disagrees in shape (partial-surface bug).
  - Hook still fires after uninstall.
  - mem_after.json non-empty.
  - install.json reports failure but a surface stays registered (rollback bug).
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### XCT-08 — SSE reconnect across daemon restart with bridge-driven prompt mid-tool

```yaml qa-scenario
id: xct-08-sse-reconnect-across-restart-bridge-tool-inflight
title: Web client SSE-subscribes; bridge inbound (Slack) starts a real Claude Code prompt that calls a long-running tool; SIGTERM mid-tool; client reconnects with `Last-Event-ID`; new daemon replays exactly the durably-appended events
theme: cross-cutting
composes:
  - DB-04
  - DB-05
  - DB-15
  - ACP-08
  - ACP-15
  - API-05
  - OBS-03
  - OBS-05
  - EXT-06
coverage:
  primary:
    - cross.sse.restart.replay.durable-append-only
  secondary:
    - bridge.delivery.session
    - tool.process.kill-on-shutdown
live: true
provider: claude-code
modules: [api.httpapi, sse, observe, daemon, session, tools, bridges]
```

```yaml qa-flow
preconditions:
  - Lab home with QA bridge fixture (Slack-class) using `internal/bridges/qa-bridge` or equivalent test bridge.
  - Real Claude Code; long tool fixture `xct__sleep` that sleeps 20 s then writes a marker.
steps:
  - run: agh daemon start && sleep 5
  - run: agh extension install ./fixtures/xct08-bridge && agh bridge configure qa-bridge --channel qa-room -o json
  - run: curl -N "http://127.0.0.1:$AGH_HTTP_PORT/api/observe/events?stream=1" >sse.log &
  - set: SSE_PID=$!
  - sleep: 2
  - run: agh bridge inject --channel qa-room --text "Use xct__sleep then reply 'done'." -o json | tee inject.json
  - sleep: 4    # tool dispatched, in-flight
  - run: PID=$(jq -r '.pid' "$LAB_HOME/daemon.json")
  - run: LAST_SEQ=$(grep -oE 'id: [0-9]+' sse.log | tail -1 | awk '{print $2}')
  - run: kill -TERM $PID
  - sleep: 12
  - run: ps -ef | grep xct__sleep | grep -v grep | wc -l   # MUST be 0 (DB-05 process group)
  - run: agh daemon start && sleep 5
  - run: curl -N -H "Last-Event-ID: $LAST_SEQ" "http://127.0.0.1:$AGH_HTTP_PORT/api/observe/events?stream=1" >sse.post.log &
  - sleep: 3
  - run: jq -s 'add | unique_by(.id)' sse.log sse.post.log > all_events.json
  - run: jq 'length' all_events.json
  - run: jq '[.[].id] | min, max' all_events.json
  - run: agh observe events --since "$(date -d '5 minutes ago' --utc -Iseconds)" -o json > durable.json
  - run: jq -s 'flatten | unique_by(.id) | length' all_events.json durable.json
expected_behavior:
  - Pre-shutdown durably-appended events are replayed exactly once on reconnect (per OBS-03 durable-append-before-broadcast and `internal/api/httpapi/middleware.go` `Last-Event-ID`).
  - No event in sse.post.log has `id <= LAST_SEQ`.
  - all_events.json `unique_by(.id) | length == length` (no duplicates).
  - tools count is 0 → process group killed during shutdown.
  - The tool that was in-flight has either: (a) a `tool_call.completed` event committed before SIGTERM AND visible after restart, or (b) a `tool_call.cancelled` event recorded with `release_reason="shutdown"`; nothing in between (no half-state).
evidence_to_capture:
  - inject.json, sse.log, sse.post.log, all_events.json, durable.json.
failure_signatures:
  - sse.post.log replays an event with `id <= LAST_SEQ` (duplicate broadcast).
  - all_events.json contains duplicates by `id`.
  - In-flight tool process still alive.
  - In-flight tool has `tool_call.started` without a paired `cancelled`/`completed` event after restart.
cleanup:
  - agh daemon stop && agh extension uninstall xct08-bridge
```
```

```markdown
### XCT-09 — Bridge inbound (Slack) → real Claude Code → tool call → bridge outbound: full actor correlation

```yaml qa-scenario
id: xct-09-bridge-realllm-tool-bidi-actor-correlation
title: A real Slack message creates a daemon session; real Claude Code answers via a tool-driven response; outbound returns to the same Slack thread; events show actor_kind=bridge,actor_id=slack:<ch> end-to-end
theme: cross-cutting
composes:
  - EXT-06
  - EXT-13
  - ACP-04
  - TOL-01
  - OBS-02
coverage:
  primary:
    - cross.bridge.realllm.bidi.actor-correlation
  secondary:
    - bridges.signature.verify
    - observe.actor.bridge
live: true
provider: claude-code
modules: [bridges, session, tools, observe, api]
```

```yaml qa-flow
preconditions:
  - Real Slack workspace + bot OR a high-fidelity QA bridge that emits identical event shape (preferred for deterministic CI; live Slack lane optional).
  - Lab home; real Claude Code.
steps:
  - run: agh daemon start && sleep 5
  - run: agh bridge configure slack --token $SLACK_BOT_TOKEN --signing-secret $SLACK_SIGNING_SECRET -o json
  - run: curl -X POST "http://127.0.0.1:$AGH_HTTP_PORT/api/bridges/slack/events" -H "X-Slack-Signature: $(./fixtures/xct09/sign.sh slack)" -H "X-Slack-Request-Timestamp: $(date +%s)" -d @./fixtures/xct09/event_message.json -o resp.json
  - sleep: 8
  - run: agh sessions list --since "$(date -d '5 minutes ago' --utc -Iseconds)" -o json | tee sess.json
  - set: SID=$(jq -r '.[0].id' sess.json)
  - run: agh observe events --session $SID -o json > events.json
  - run: jq '[.[] | {event_type, actor_kind, actor_id, agent_name, root_session_id}]' events.json | tee summary.json
  - run: jq '.[] | select(.event_type=="bridge_outbound")' events.json | tee outbound.json
  - run: jq -r '.thread_ts // .channel' outbound.json | tee out_addr.txt
  - run: jq -r '.thread_ts // .channel' ./fixtures/xct09/event_message.json | tee in_addr.txt
  - run: cmp out_addr.txt in_addr.txt
expected_behavior:
  - At least one `bridge_inbound`, one `session_started`, one `tool_call`, one `agent_message`, one `bridge_outbound` for $SID (OBS-02 coverage matrix).
  - Every event has `actor_kind=bridge` and `actor_id=slack:<channel>` per `internal/CLAUDE.md:49`.
  - Inbound and outbound share the Slack thread address (cmp passes).
  - signature verification path traced — invalid signature variant rejected with 401 (EXT-13).
evidence_to_capture:
  - resp.json, sess.json, events.json, summary.json, outbound.json, addr files.
failure_signatures:
  - Outbound delivered to a different thread.
  - Any event missing `actor_kind` / `actor_id`.
  - `agent_name` empty on `tool_call`.
  - Signature path bypassed (any unsigned request accepted).
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### XCT-10 — Multi-workspace correctness: A and B run different agents simultaneously without leakage

```yaml qa-scenario
id: xct-10-multi-workspace-isolation-end-to-end
title: Two workspaces (A, B) run real Claude Code sessions concurrently; memory cannot cross; settings overlay differs by workspace; web switcher renders both; correlation keys carry workspace_id everywhere
theme: cross-cutting
composes:
  - cfg-06
  - cfg-08
  - MEM-11
  - MEM-12
  - ACP-04
  - API-04
  - SKL-13
coverage:
  primary:
    - cross.workspace.isolation.memory-settings-sessions-web
  secondary:
    - workspace.resolver.modes
    - memory.scope.workspace
live: true
provider: claude-code
modules: [workspace, config, settings, memory, session, web, api]
```

```yaml qa-flow
preconditions:
  - Lab home; two workspaces under `./fixtures/xct10-A/` and `./fixtures/xct10-B/`; both have AGENT.md + SOUL.md.
  - Real Claude Code on both.
steps:
  - run: agh daemon start && sleep 5
  - run: agh workspace add ./fixtures/xct10-A --name wsA -o json
  - run: agh workspace add ./fixtures/xct10-B --name wsB -o json
  - run: agh config set --workspace wsA acp.subprocess.timeout_seconds 60 -o json
  - run: agh config set --workspace wsB acp.subprocess.timeout_seconds 30 -o json
  - run: agh sessions start --agent claude-code --workspace wsA -o json | tee a.json &
  - run: agh sessions start --agent claude-code --workspace wsB -o json | tee b.json &
  - wait
  - set: SA=$(jq -r '.id' a.json) SB=$(jq -r '.id' b.json)
  - run: agh sessions prompt $SA --message "Remember the secret 'AAAA-only' (workspace memory)." -o json
  - run: agh sessions prompt $SB --message "Remember the secret 'BBBB-only' (workspace memory)." -o json
  - sleep: 6
  - run: agh sessions prompt $SA --message "What was the secret in this workspace?" -o json | tee a_recall.json
  - run: agh sessions prompt $SB --message "What was the secret in this workspace?" -o json | tee b_recall.json
  - run: ! grep -q 'BBBB' a_recall.json
  - run: ! grep -q 'AAAA' b_recall.json
  - run: agh observe events --session $SA -o json | jq -r '.[].workspace_id' | sort -u | tee a_ws.txt
  - run: agh observe events --session $SB -o json | jq -r '.[].workspace_id' | sort -u | tee b_ws.txt
  - run: cmp a_ws.txt <(echo "wsA") && cmp b_ws.txt <(echo "wsB")
  - run: curl -s "http://127.0.0.1:$AGH_HTTP_PORT/api/workspaces" | jq -r '.[].name' | sort | tee web_ws.txt
  - run: grep -q wsA web_ws.txt && grep -q wsB web_ws.txt
expected_behavior:
  - Recall in A produces "AAAA-only" but never "BBBB-only" (and vice versa); MEM-11/MEM-12 isolation.
  - Settings overlay differs: `agh config get --workspace wsA acp.subprocess.timeout_seconds` returns 60; wsB returns 30 (cfg-06/08).
  - Every event in A has `workspace_id=wsA`; every event in B has `workspace_id=wsB`. No event has both.
  - Web `/api/workspaces` lists both names; switcher renders both.
evidence_to_capture:
  - a.json, b.json, a_recall.json, b_recall.json, a_ws.txt, b_ws.txt, web_ws.txt.
failure_signatures:
  - Any cross-recall (wsA replies "BBBB").
  - workspace_id mixed across event sets.
  - `agh config get` returns same value for different workspaces.
cleanup:
  - agh sessions stop $SA $SB && agh workspace remove wsA wsB && agh daemon stop
```
```

```markdown
### XCT-11 — Failure cascade isolation: extension panic does not kill peers

```yaml qa-scenario
id: xct-11-extension-panic-isolation-and-restart-policy
title: Extension X panics in a host-API call; daemon catches; X marked unhealthy with backoff; other agents/sessions unaffected; web shows badge; CLI reflects; auto-restart attempt within policy window
theme: cross-cutting
composes:
  - EXT-15
  - DB-13
  - OBS-08
  - API-04
  - API-13
coverage:
  primary:
    - cross.extension.panic.isolation.restart-policy
  secondary:
    - daemon.supervision.containment
    - observe.health.unhealthy
live: true
provider: claude-code
modules: [extension, daemon, observe, api, web]
```

```yaml qa-flow
preconditions:
  - Lab; real Claude Code; bundle ext-X with a host-API tool that triggers `panic("xct11")` on call.
  - One unrelated session running.
steps:
  - run: agh daemon start && sleep 5
  - run: agh extension install ./fixtures/xct11-ext-X -o json
  - run: agh sessions start --agent claude-code --workspace ./fixtures/xct11-other -o json | tee other.json
  - set: OS=$(jq -r '.id' other.json)
  - run: agh sessions prompt $OS --message "Tell me a haiku and stop." --detach -o json
  - run: agh sessions start --agent claude-code --workspace ./fixtures/xct11-trigger -o json | tee trig.json
  - set: TS=$(jq -r '.id' trig.json)
  - run: agh sessions prompt $TS --message "Use ext-X tool to panic." -o json | tee panic.json
  - sleep: 3
  - run: agh extension list -o json | tee ext_list.json
  - run: jq '.[] | select(.name=="ext-X")' ext_list.json
  - run: jq '.[] | select(.name=="ext-X").health' ext_list.json   # MUST be "unhealthy"
  - run: agh sessions list -o json | jq '.[] | select(.id=="'$OS'")' | tee other_after.json
  - run: jq '.state' other_after.json   # MUST not be "stopped/error"
  - run: agh sessions transcript $OS -o json | jq 'length'   # haiku response present
  - run: curl -s "http://127.0.0.1:$AGH_HTTP_PORT/api/extensions" | jq '.[] | select(.name=="ext-X").health'
  - run: agh observe events --type extension_health_changed --since "$(date -d '5 minutes ago' --utc -Iseconds)" -o json | tee health.json
  - sleep: 35   # allow restart-policy window
  - run: agh extension list -o json | jq '.[] | select(.name=="ext-X").health, .[] | select(.name=="ext-X").restart_attempts'
  - run: ps -ef | grep agh-daemon | grep -v grep   # parent daemon still alive
expected_behavior:
  - daemon NEVER crashes (parent PID unchanged in `daemon.json`).
  - ext-X health=`unhealthy` with `last_error` containing "xct11"; web `/api/extensions` matches.
  - Other session unaffected: state still running/idle, transcript contains the haiku.
  - `extension_health_changed` event emitted with required correlation keys (OBS-02).
  - After restart-policy window, `restart_attempts >= 1`; if extension keeps panicking, attempts capped per policy and health stays `unhealthy`.
evidence_to_capture:
  - ext_list.json, panic.json, other_after.json, health.json.
failure_signatures:
  - daemon PID changed (panic propagated).
  - other_after.json shows `error`.
  - ext-X health stays `healthy` after panic (containment failure).
  - restart_attempts grows unboundedly with no backoff cap.
cleanup:
  - agh extension uninstall ext-X && agh sessions stop $OS $TS && agh daemon stop
```
```

```markdown
### XCT-12 — Greenfield zero-legacy invariant: fresh AGH_HOME has no migration paths for old state, no deleted-feature names anywhere

```yaml qa-scenario
id: xct-12-greenfield-zero-legacy-invariant
title: A brand-new AGH_HOME has no migration shim code on the boot path; deleted-feature vocabulary (recipe, playbook, etc.) is genuinely absent from the running binary's surfaces
theme: cross-cutting
composes:
  - DB-01
  - cfg-09
  - SD-002 (standing directive)
  - L-006 (lesson)
coverage:
  primary:
    - cross.greenfield.zero-legacy
  secondary:
    - daemon.boot.fresh-home
    - vocabulary.glossary
live: false
provider: none
modules: [daemon, store, cli, api, docs]
```

```yaml qa-flow
preconditions:
  - Brand-new AGH_HOME (`rm -rf $LAB_HOME && mkdir -p $LAB_HOME`).
steps:
  - run: agh daemon start && sleep 5
  - run: grep -i 'migrating from\|legacy\|backward' "$LAB_HOME/logs/agh.log"; echo $?   # MUST be 1
  - run: agh --help 2>&1 | tee cli.txt
  - run: grep -iE '\b(recipe|playbook|workflow-task|procedure)\b' cli.txt; echo $?   # MUST be 1
  - run: curl -s "http://127.0.0.1:$AGH_HTTP_PORT/openapi.json" > openapi.json
  - run: grep -iE '\b(recipe|playbook|workflow-task|procedure)\b' openapi.json; echo $?
  - run: find ./web/dist -name '*.js' -o -name '*.html' 2>/dev/null | xargs -r grep -lE '\brecipe\b' || true
  - run: find ./packages/site/.next ./packages/site/out -type f 2>/dev/null | xargs -r grep -lE '\brecipe\b' | tee site_hits.txt
  - run: test ! -s site_hits.txt
  - run: agh observe events --since "$(date -d '5 minutes ago' --utc -Iseconds)" -o json | jq '.[] | select(.event_type | test("legacy|migrate"))'
expected_behavior:
  - boot log free of migration prose for "old state" (greenfield-delete rule, SD-002 + L-006).
  - CLI help, OpenAPI spec, web bundle, docs site contain zero hits for `recipe|playbook|workflow-task|procedure` in the canonical AGH meaning per `docs/_memory/glossary.md`.
  - No event has `event_type` matching `legacy|migrate`.
evidence_to_capture:
  - cli.txt, openapi.json, site_hits.txt, boot agh.log excerpt.
failure_signatures:
  - Boot log mentions "migrating from" / "legacy".
  - Any deleted vocabulary appears in OpenAPI / CLI / web.
cleanup:
  - agh daemon stop && rm -rf $LAB_HOME
```
```

```markdown
### XCT-13 — Real lifecycle hook chain across three layers; one hook errors → fail-open

```yaml qa-scenario
id: xct-13-lifecycle-hook-chain-three-layers-fail-open
title: Bundle B installs hooks at three layers; session start fires them in hierarchy then alphabetical order; one hook errors → fail-open; subsequent hooks still run; ledger captures all timings
theme: cross-cutting
composes:
  - EXT-14
  - MEM-09
  - MEM-10
  - SKL-03
  - OBS-01
  - OBS-02
coverage:
  primary:
    - cross.hooks.lifecycle-chain.fail-open
  secondary:
    - hooks.ordering.hierarchy-then-alphabetical
    - extension.bundle.install
live: true
provider: claude-code
modules: [hooks, skills, extension, session, observe]
```

```yaml qa-flow
preconditions:
  - Lab; real Claude Code.
  - Bundle layout: bundled hook `bundled/zzz-last`, marketplace hook `marketplace/aaa-first`, marketplace hook `marketplace/mmm-mid` (errors), workspace hook `workspace/qqq-after`.
  - Hook event: `on_session_created`. Default 5 s timeout; fail-open semantics.
steps:
  - run: agh daemon start && sleep 5
  - run: agh extension install ./fixtures/xct13-bundle -o json
  - run: agh sessions start --agent claude-code --workspace ./fixtures/xct13 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - sleep: 8   # allow hooks to settle
  - run: agh observe events --session $SID --type hook_invoked -o json | tee hooks.json
  - run: jq '[.[] | {hook_name, status, duration_ms, sequence}]' hooks.json | tee summary.json
  - run: jq -r '.[].hook_name' hooks.json | tee order.txt
expected_behavior:
  - order.txt is exactly:
      bundled/zzz-last        ← bundled layer first per hierarchy precedence
      marketplace/aaa-first
      marketplace/mmm-mid
      workspace/qqq-after     ← workspace last
    Within a layer, alphabetical.
  - hooks.json: `mmm-mid` has `status:"error"`, `error.code:"hook_runtime"`; subsequent hooks still ran (`workspace/qqq-after.status="ok"`); fail-open semantics per MEM-10.
  - summary.json: every hook has a `duration_ms` and a `sequence`.
evidence_to_capture:
  - sess.json, hooks.json, summary.json, order.txt.
failure_signatures:
  - order.txt mismatches hierarchy or alphabetical inside a layer.
  - workspace/qqq-after did not run after marketplace/mmm-mid errored (fail-closed bug).
  - Any hook missing `duration_ms` (timing not captured).
cleanup:
  - agh sessions stop $SID && agh extension uninstall xct13-bundle && agh daemon stop
```
```

```markdown
### XCT-14 — Truthful UI gate: web ahead of daemon must fail QA

```yaml qa-scenario
id: xct-14-truthful-ui-gate-rejects-frontend-ahead
title: Deliberately add a web control whose backend endpoint does not exist; QA gate must reject the build
theme: cross-cutting
composes:
  - SD-007 (standing directive)
  - API-07
  - API-08
  - API-15
coverage:
  primary:
    - cross.truthful-ui.frontend-ahead-rejected
  secondary:
    - api.contract.no-partial-surface
live: false
provider: none
modules: [web, api.contract, api.httpapi, codegen, ci]
```

```yaml qa-flow
steps:
  - run: cd /tmp && cp -r $REPO/web ./web-fork && cd ./web-fork
  - run: # add a button calling fetch('/api/xct/nonexistent') in src/components/__qa__/PhantomButton.tsx
  - run: cd $REPO && bun --cwd ./web run build 2>&1 | tee build.log
  - run: ./scripts/qa/truthful-ui-gate.sh ./web-fork 2>&1 | tee gate.log
  - assert: gate.log contains "fetch path /api/xct/nonexistent has no matching route in internal/api/contract"
  - assert: gate exit code != 0
  - run: # restore web/ unchanged
expected_behavior:
  - The truthful-UI gate compares all `fetch(...)` paths in the web bundle against the OpenAPI route table; mismatches FAIL the build (per SD-007).
  - Gate fails on the planted fetch.
evidence_to_capture:
  - build.log, gate.log.
failure_signatures:
  - Gate passes despite the planted phantom fetch.
  - Gate output does not name the offending path.
cleanup:
  - rm -rf /tmp/web-fork
```
```

```markdown
### XCT-15 — Codegen co-ship: contract edit then codegen-check then codegen lockstep

```yaml qa-scenario
id: xct-15-codegen-coship-contract-openapi-ts
title: Edit a contract field; codegen-check fails; run codegen; both openapi/agh.json and web/src/generated/agh-openapi.d.ts updated; codegen-check now passes; web TypeScript build green
theme: cross-cutting
composes:
  - API-07
  - API-08
  - API-09
  - L-007 (lesson)
coverage:
  primary:
    - cross.codegen.contract-openapi-ts-coship
  secondary:
    - codegen.drift.detection
live: false
provider: none
modules: [api.contract, codegen, web, ci]
```

```yaml qa-flow
steps:
  - run: cd $REPO
  - run: cp internal/api/contract/sessions.go internal/api/contract/sessions.go.bak
  - run: # add field `XCT15Field string \`json:"xct15_field,omitempty\"\`` to a request type in sessions.go
  - run: make codegen-check 2>&1 | tee codegen-check.before.log
  - assert: codegen-check.before.log contains "drift detected" AND exit code != 0
  - run: make codegen 2>&1 | tee codegen.log
  - assert: openapi/agh.json contains "xct15_field"
  - assert: web/src/generated/agh-openapi.d.ts contains "xct15_field"
  - run: make codegen-check 2>&1 | tee codegen-check.after.log
  - assert: codegen-check.after.log exit code == 0
  - run: make bun-typecheck 2>&1 | tee tsc.log
  - assert: tsc.log exit code == 0
  - run: mv internal/api/contract/sessions.go.bak internal/api/contract/sessions.go && make codegen
expected_behavior:
  - Drift is detected before codegen.
  - codegen updates both artifacts in lockstep (per agh-contract-codegen-coship skill, L-007).
  - TypeScript build remains green.
evidence_to_capture:
  - codegen-check.before.log, codegen.log, codegen-check.after.log, tsc.log.
  - Diff of openapi/agh.json + web/src/generated/agh-openapi.d.ts before/after the edit.
failure_signatures:
  - codegen-check.before.log exit code 0 (drift undetected).
  - Either artifact updated alone (lockstep broken).
  - TS build red after codegen (handler/web mismatch).
cleanup:
  - git restore internal/api/contract/sessions.go openapi/agh.json web/src/generated/agh-openapi.d.ts
```
```

```markdown
### XCT-16 — Standing directive SD-005 audit: every module child has at least one real-LLM scenario

```yaml qa-scenario
id: xct-16-sd-005-real-llm-coverage-audit
title: A repo-level audit script enforces SD-005 — every QA child accepted as release-ready has at minimum one `qa-scenario` with `live: true` and a real provider
theme: cross-cutting
composes:
  - SD-005 (standing directive)
  - 01-daemon-boot
  - 02-config-settings
  - 03-acp-sessions
  - 04-autonomy-kernel
  - 05-memory-soul
  - 06-skills-capabilities
  - 07-tools-sandbox
  - 08-extensions-bridges
  - 09-automation-cron
  - 10-network-identity
  - 11-api-cli-parity
  - 15-observability
coverage:
  primary:
    - cross.qa.coverage.sd-005
live: false
provider: none
modules: [qa.framework, ci]
```

```yaml qa-flow
steps:
  - run: ./scripts/qa/sd005-audit.sh ./.compozy/tasks/final-qa/_children/ 2>&1 | tee sd005.log
  - assert: every file under _children/ either has `module: cross-cutting` (this child) OR contains at least one `live: true` scenario with `provider: claude-code|openclaw|hermes`.
  - assert: sd005.log lists `release-ready: yes|no` per child.
  - run: jq -e 'all(.[]; .release_ready==true)' < sd005.log
expected_behavior:
  - sd005.log shows release-ready=yes for every module child.
  - No false negatives (the audit must include this cross-cutting child correctly: it satisfies SD-005 by composing on other live scenarios).
evidence_to_capture:
  - sd005.log; the script source under scripts/qa/sd005-audit.sh.
failure_signatures:
  - Any child with no `live:true, provider:claude-code|openclaw|hermes` scenario.
  - Audit reports release-ready=yes for a child missing live coverage.
cleanup: none
```
```

## 5. Edge Cases (gates, not full scenarios)

These cross-module edges must be exercised as short asserts inside the cross-cutting QA gate, not as full scenarios:

- **Two daemons boot against same provider home but different AGH_HOME** — must NOT corrupt provider auth files. Per the provider-home isolation rule (`CLAUDE.md` provider-home isolation), each lab uses a distinct `PROVIDER_HOME` from `bootstrap-manifest.json`; assert via fingerprint of `PROVIDER_CODEX_HOME` directories.
- **Concurrent `agh config set` against the same isolated home** — forbidden by CLAUDE.md ("Never parallelize config writes against one isolated QA home"). Gate: launch two `agh config set` in parallel; one must lose with a clear error and no file half-written; assert via `cfg-14`.
- **Extension declares both a CLI verb AND no UDS verb** (or any other partial surface) — `internal/extension/manifest.go` must reject; assert via EXT-01 + EXT-11 composition.
- **Cron-fired session inherits agent `actor_kind=automation` only when delivery=announce; for delivery=child it is `actor_kind=session, parent=automation:<cron-id>`** — assert per CRN-14.
- **Hook `pre_tool_use` deny short-circuits BEFORE provider-side spawn** — composition of AUT-03 + TOL-14: assert no provider subprocess starts on deny (no extra `subprocess.spawn` event for that tool call).
- **Two bundles claiming the same primary channel** — second activation rejected (EXT-20). Cross-cutting twist: assert that the failed activation does NOT leave half-installed artifacts in `internal/skills/registry`.
- **Extension uninstall during in-flight tool call** — must wait for the in-flight call to settle OR cancel cleanly with `release_reason="extension_uninstalled"`; never orphan a tool process. Gate: dispatch a long tool, uninstall, assert tool process killed within timeout.
- **Memory write at workspace scope from a coordinator-spawned child** — `workspace_id` must be the child's workspace, not the coordinator's. Gate: AUT-09 + MEM-11.
- **Settings hot-apply during an active prompt** — should NOT cancel the in-flight prompt (detached lifetime, L-001). Gate: cfg-04 + ACP-08 composition.
- **`make codegen-check` after `make codegen` is idempotent** — running it twice in a row must not introduce drift. Gate as part of XCT-15.
- **Web build with stale generated TS** — `web/src/generated/agh-openapi.d.ts` older than `internal/api/contract/*.go` mtime should make `make web-build` fail or `make codegen-check` catch it.
- **Concurrent worktree QA** — each lab MUST have unique `AGH_HOME`, daemon ports, UDS path, Web proxy target, and tmux-bridge socket; assert by reading two manifests and confirming no overlap.

## 6. Integration Surfaces

Cross-cutting obligations between modules. This is a different table from the per-child surface tables — it lists invariants that compose across ≥3 modules.

| Invariant | Modules required to agree | Citation |
| --- | --- | --- |
| `claim_token_hash` on the wire; raw `claim_token` never on the wire | `internal/task`, `internal/network`, `internal/observe`, `internal/api/contract`, `internal/diagnostics` | `internal/CLAUDE.md:55`, `internal/api/contract/agents.go:479-498`, `internal/network` ingress reject (NET-05) |
| `root_session_id` correlation across cron, network, bridges | `internal/automation`, `internal/network`, `internal/bridges`, `internal/session`, `internal/observe` | `internal/CLAUDE.md:49` correlation keys |
| Detached lifetime: prompts, network sends, automation jobs, bridge dispatches | `internal/session`, `internal/network`, `internal/automation`, `internal/bridges`, `internal/extension/host_api.go:1724` | `internal/CLAUDE.md:33-35`, L-001 |
| Hook dispatch at the call site, never tail event tables | `internal/hooks`, `internal/session`, `internal/extension`, `internal/tools`, `internal/skills` | `internal/CLAUDE.md:24-26` |
| Authoritative-primitive exclusivity: only `task.Service.ClaimNextRun` claims | `internal/task`, `internal/scheduler`, `internal/coordinator`, `internal/automation` | L-005, AUT-07/AUT-14 |
| Codegen co-ship: contract → openapi → web TS in a single change | `internal/api/contract`, `openapi/agh.json`, `web/src/generated`, `make codegen-check` | `agh-contract-codegen-coship` skill, L-007 |
| Truthful-UI > plausible-UI | `web/`, `internal/api/contract`, packages/site | SD-007 |
| Composition-root discipline: no package imports `daemon/`, `api/`, `cli/` | `internal/daemon/boundary.go`, `magefile.go` Boundaries | SD-008 |
| Greenfield-delete: no migration shims for "old state" | every package with persisted state, `docs/_memory/glossary.md` vocabulary | SD-002, L-006 |
| Extensible + agent-manageable: every capability has CLI/HTTP/UDS parity AND extension surface | `internal/cli`, `internal/api/httpapi`, `internal/api/udsapi`, `internal/extension`, docs/site | SD-011 |
| Real-scenario QA: every release pass has ≥1 live scenario per module | `final-qa/_children/*.md` | SD-005 |
| Worktree isolation: unique AGH_HOME, daemon/UDS/Web endpoints, tmux-bridge socket | `agh-qa-bootstrap` skill, `agh-worktree-isolation` skill | L-009, CLAUDE.md worktree rule |

## 7. DX Cliffs

Cross-cutting failure modes that single-module DX cliffs miss.

1. **Operator runs `agh extension install` and the web UI doesn't refresh** — symptom: install reports success, CLI lists the new extension, but the web tile doesn't appear until the operator hard-reloads. Repro: install bundle, watch web. Fix surface: `internal/extension/manager.go` must broadcast `extension_installed` to the SSE bus AFTER all four surfaces are registered (XCT-07 +EXT-11).

2. **`agh sessions resume` after a daemon crash silently produces a "fresh" session if the prior `claim_token_hash` was lost** — repro per DB-04 + AUT-08. Fix surface: `bootSessionRepair` (`internal/daemon/boot.go:545-617`) must reconcile `task_runs.claim_token_hash` BEFORE marking sessions resumable.

3. **Cron firing while consolidation is running can starve the consolidation Lock gate** — repro: dense one-minute cron + small workspace sessions; consolidation Time gate hits, but Lock gate fails to acquire because cron-driven sessions are still writing memory. Fix surface: `internal/memory/consolidation/runtime.go` Lock acquire must back off, not abandon, with telemetry.

4. **Two-instance network: B's audit log carries A's `agent_id` but not A's `peer_card_fingerprint`** — repro: NET-19 + XCT-06; without the fingerprint the audit row cannot be validated post-incident. Fix surface: `internal/network` audit emitter must include the verified peer card fingerprint.

5. **Extension lifecycle hook errors are swallowed silently in fail-open mode** — repro: XCT-13 with `mmm-mid` erroring; ledger captures but operator never sees a UI/CLI badge. Fix surface: emit a separate `hook_runtime_error` event consumed by the web "alerts" panel.

6. **Codegen drift errors do not name the offending field** — repro: edit one of 200 lines; `make codegen-check` says "drift detected" without showing where. Fix surface: `make codegen-check` should print a structured diff with the field path.

7. **Bridge inbound that maps to no agent fails into the void** — repro: send Slack message to a channel where no agent is mapped; nothing happens, no log line at the daemon level. Fix surface: `internal/bridges` must emit a `bridge_inbound_unmapped` event with the channel id.

8. **`agh daemon stop` while a real cron is firing in the same minute** — repro: CRN-04 + DB-05; the firing session may be torn down with `release_reason="shutdown"` but the cron entry's last_run timestamp still updates, leading the next start to think the run completed. Fix surface: cron must mark `last_run_status=interrupted` when shutdown happens between fire and completion.

9. **`webOpenPage`-style daemon-side browser helpers absent** — repro: openclaw uses Control-UI roundtrip via daemon-side browser RPC; AGH today requires Playwright as a separate harness. Fix surface: TBD; recommend daemon-side `browser.*` RPC for `final-qa/_children/*` to drive web UI inside the same scenario flow.

10. **Workspace-id mismatch on coordinator-spawned task** — repro: AUT-09 + MEM-11; if the coordinator's spawn omits `workspace_id`, the child session inherits the coordinator's workspace and writes memory there. Fix surface: `internal/coordinator` MUST set `workspace_id` on every spawn payload.

## 8. Failure Modes QA Must Catch

Cross-cutting failures that, if shipped, indicate broken module composition. Each is a release-blocker.

1. **A daemon restart silently drops a real-LLM session's transcript continuity.** XCT-01 must always show the post-restart prompt threaded into the same session_id with strictly monotonic sequences.
2. **Bundle install reports success but a surface is missing.** XCT-02 / XCT-07: install.json `surfaces` must equal the union of skills∪tools∪hooks∪commands declared in `manifest.toml`; uninstall must remove all four atomically.
3. **A hook runs at the wrong call site.** XCT-02 + XCT-13: deny must short-circuit BEFORE provider spawn; ordering across layers must be hierarchy-then-alphabetical with deterministic ledger.
4. **Cron→child→real-LLM→consolidation chain breaks at any seam.** XCT-04: touched_count never incrementing → consolidation gate dead. cron events without `parent_session_id` for `delivery=child` → lineage broken.
5. **Coordinator claims using something other than `task.Service.ClaimNextRun`.** XCT-05 + L-005: any peer claim path is a structural redesign trigger (two-touch rule).
6. **Network leaks raw `claim_token`.** XCT-06: greedy regex `\bagh_claim_[A-Za-z0-9]+\b` over all logs/audits/SSE on both labs MUST return zero matches.
7. **SSE replay across restart drops or duplicates events.** XCT-08 + OBS-03: durable-append-before-broadcast invariant must hold even with a tool in-flight at SIGTERM.
8. **Bridge correlation breaks on outbound delivery.** XCT-09: outbound MUST land in the same Slack thread as the inbound; `actor_kind/actor_id` carries through every event.
9. **Workspace memory crosses workspaces.** XCT-10: any cross-recall is release-blocking.
10. **Extension panic crashes the daemon.** XCT-11: parent PID must NOT change.
11. **Greenfield rule is violated.** XCT-12: any deleted vocabulary in CLI/OpenAPI/web/site or any "migrating from" log message blocks release.
12. **Truthful-UI gate misses a frontend-ahead state.** XCT-14: planted phantom fetch must FAIL the build.
13. **Codegen drift undetected.** XCT-15: `codegen-check.before.log` MUST exit non-zero on a contract edit before `make codegen` runs.
14. **A module child has no real-LLM scenario.** XCT-16: SD-005 audit must enforce.

## 9. Fixtures / Bootstrap Requirements

Cross-cutting QA harness needs, in addition to single-module child requirements:

- **Parallel-lab bootstrap**: `agh-qa-bootstrap` must produce non-overlapping AGH_HOME, daemon/UDS/Web endpoints, tmux-bridge sockets, and provider homes. XCT-06 itself uses one fresh lab because Network delivery is installation-local.
- **QA bridge fixture**: an in-process bridge (Slack-class) that emits `bridge_inbound`/`bridge_outbound` events with the same shape as the real Slack bridge; gated behind a build tag so production binary cannot ship it. Used by XCT-08, XCT-09 (CI default), with the live Slack lane optional.
- **Long-tool fixture**: `xct__sleep` that sleeps configurable duration; required for XCT-08.
- **Panic-extension fixture**: `xct11-ext-X` with a host-API call that triggers `panic("xct11")`; required for XCT-11. Build-tag-gated.
- **Phantom-fetch web fork**: `scripts/qa/truthful-ui-gate.sh` runs against an arbitrary web tree; required for XCT-14. Implementation: parse all `fetch(...)` literals, parse `openapi/agh.json` route table, fail on mismatches.
- **SD-005 audit**: `scripts/qa/sd005-audit.sh ./.compozy/tasks/final-qa/_children/`; required for XCT-16.
- **Codegen drift script**: `make codegen-check` already exists; XCT-15 tests it via a structured edit/restore loop.
- **Greenfield grep**: `scripts/qa/greenfield-grep.sh` reads `docs/_memory/glossary.md` for "deleted-vocabulary" patterns and runs them against CLI help, OpenAPI, web bundle, site build. Required for XCT-12.
- **Cross-cutting artifact layout**: per scenario `.artifacts/qa/<run-id>/cross-cutting/xct-<NN>/` with the standard four files (`qa-report.md`, `qa-summary.json`, `qa-output.log`, `qa-observed-events.json`). Plus a top-level `xct-summary.json` aggregating all 16 scenarios for the SD-005 audit.
- **Bakeoff order**: cross-cutting scenarios are mostly LLM-symmetric; the live ones run GPT-class (Claude Code default), then OpenClaw, then Hermes — same order as the per-module bakeoff. XCT-15 and XCT-12 are LLM-free (`live: false`).
- **Two-touch rule trip wire**: when XCT-04 or XCT-08 fail twice in a release window, escalate to a TechSpec instead of a third patch (per CLAUDE.md two-touch rule).

## 10. Citations

- `CLAUDE.md` (root) — greenfield-delete (SD-002 echo), commit style, worktree isolation, provider-home isolation, codegen co-ship, two-touch rule.
- `internal/CLAUDE.md:24-37,49-57,124-129` — composition-root discipline, hooks-at-call-site, detached lifetime, correlation keys, claim_token redaction, lifecycle hooks ordering.
- `internal/daemon/daemon.go:1075-1318` — Daemon.Run / Shutdown sequencing; SIGINT/SIGTERM bridge; section reused by XCT-01, XCT-08.
- `internal/daemon/boot.go:152-1779` — composition-root boot pipeline; bootCleanup LIFO; bootSessionRepair (545-617); used in XCT-01, XCT-04.
- `internal/api/httpapi/middleware.go:45-47` — `Last-Event-ID` exposed; required for XCT-01, XCT-08.
- `internal/api/httpapi/httpapi_integration_test.go:417,1511` — existing reference for `Last-Event-ID` and `after_sequence` SSE replay.
- `internal/task/types.go:276,380`, `internal/task/validate.go:488-497` — `claim_token_hash` is mandatory when `claim_token`/`lease_until`/`heartbeat_at` is set; XCT-05, XCT-06.
- `internal/extension/host_api.go:1724` — `context.WithoutCancel` site for prompt detachment; XCT-01, XCT-08.
- `internal/extension/manager.go:1392` — extension MutationActorKindExtension; XCT-07.
- `internal/extension/host_api_authored_context.go:824` — `heartbeat.ActorKindExtension`; XCT-13.
- `internal/coordinator/coordinator.go` — coordinator bootstrap (composed in XCT-05).
- `internal/memory/consolidation/runtime.go:437` — Lock gate uses `context.WithoutCancel`; XCT-04.
- `internal/network/` — peer card, channel publish, claim_token redaction; XCT-06.
- `internal/bridges/` — bridge runtime, signature verification, registry; XCT-09, XCT-02, XCT-08.
- `internal/observe/` — canonical event store, redaction, query engine; every XCT scenario.
- `internal/skills/registry.go` + `internal/skills/loader.go` — skill registry hot install; XCT-02, XCT-13.
- `internal/hooks/ordering.go` — hierarchy-then-alphabetical ordering; XCT-02, XCT-13.
- `internal/api/contract/authored_context_test.go:127`, `internal/api/contract/agents_test.go:161` — contract assertions on `claim_token_hash`; XCT-05, XCT-06.
- `docs/_memory/standing_directives.md`:
  - SD-002 — Remove Legacy Alpha Compatibility Code; XCT-12.
  - SD-005 — Real-Scenario QA Before Release; XCT-16.
  - SD-007 — Truthful UI > Plausible UI; XCT-14.
  - SD-008 — Composition Root Discipline; underpins every cross-cutting scenario.
  - SD-010 — Detached Execution Lifetime; XCT-01, XCT-08.
  - SD-011 — Extensible and Agent-Manageable by Design; XCT-07.
- `docs/_memory/lessons/L-001-detached-prompt-lifetime.md` — XCT-01, XCT-08.
- `docs/_memory/lessons/L-005-authoritative-primitive-exclusivity.md` — XCT-05.
- `docs/_memory/lessons/L-006-greenfield-delete-not-adapt.md` — XCT-12.
- `docs/_memory/lessons/L-007-e2e-follows-runtime-contract.md` — XCT-15.
- `docs/_memory/lessons/L-009-concurrent-worktree-deadlock.md` — Fixtures §9.
- `docs/_memory/_synthesis.md` — cross-source evidence corpus; informs every "release-blocker" gate in §8.
- `docs/_memory/glossary.md` — canonical vocabulary used by XCT-12 grep.
- `_references/openclaw-qa-patterns.md` — scenario shape, four-artifact contract, live-vs-mock policy.
- `_references/hermes-qa-patterns.md` — hermetic env, no-credential-leak fixtures.
- Sibling children for every `composes:` reference: `01-daemon-boot.md` .. `15-observability.md` (per the frontmatter).

## 11. Cross-References

- **Daemon + Boot**: `01-daemon-boot.md` — DB-04, DB-05, DB-10, DB-13, DB-14, DB-15 are direct co-actors.
- **Config + Settings**: `02-config-settings.md` — cfg-04/05/06/08/10/14.
- **ACP + Sessions**: `03-acp-sessions.md` — ACP-04/05/08/12/15/19.
- **Autonomy Kernel**: `04-autonomy-kernel.md` — AUT-01/03/04/05/09/12/15/16/18.
- **Memory + Soul**: `05-memory-soul.md` — MEM-01/04/09/10/11/19.
- **Skills + Capabilities**: `06-skills-capabilities.md` — SKL-01/03/09/11/13.
- **Tools + Sandbox**: `07-tools-sandbox.md` — TOL-01/03/08/10/14/15.
- **Extensions + Bridges**: `08-extensions-bridges.md` — EXT-01/02/03/06/11/14/15/16.
- **Automation + Cron**: `09-automation-cron.md` — CRN-01/04/05/14/18.
- **Network + Identity**: `10-network-identity.md` — NET-01/02/05/12/19.
- **API ↔ CLI parity**: `11-api-cli-parity.md` — API-02/05/07/08/09/13/17.
- **Observability**: `15-observability.md` — OBS-01/02/03/05/06/18.

## 12. Cross-Module Failure Matrix

Rows = module that misbehaves silently. Columns = module that trusts the row. Cells = "what breaks if row silently misbehaves while column trusts it". Empty cells = no direct trust seam (or self-row).

| ↓ misbehaves \ trusts → | daemon | session | task | scheduler | hooks | memory | skills | tools | extension | automation | network | observe | api(http/uds) | cli | web |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| **daemon** | — | session manager never shuts down; orphan ACP procs (DB-05) | task lease sweep dead → `claim_token_hash` rows with `lease_until<now` linger | scheduler wakeups never fire after restart | hooks dispatched in shutdown order leak | memory consolidation Lock gate stays held | skills watcher silently dies | tool-process registry orphans grow unbounded | extension Stop never called → host API leaks goroutines | automation cron runs duplicate after restart (CRN-04) | network channel sends panic on shutdown | observe broadcaster outlives store | http/uds accept connections during shutdown (DB-15) | `agh status` reports `running` indefinitely | web SSE shows stale events post-shutdown |
| **session** | daemon shutdown blocks on `WaitForFinalizations` | — | task hand-off skips claim → ClaimNextRun race (XCT-05) | scheduler thinks idle but session is busy | session-stopped hook never fires (MEM-09) | memory write at wrong scope (XCT-10) | skill activation list stale | tool dispatch routed to ghost session | extension hook receives wrong `session_id` | automation child links to wrong parent | network channel correlation broken (XCT-06) | events lack `session_id`/`workspace_id` | session APIs return 404 on resume | `agh sessions resume` fresh-starts silently (XCT-01) | web session list desyncs |
| **task** | task_runs leak past restart | session can't claim next | — | scheduler wakes but no run to claim | hook `pre_claim` skipped | — | — | tool dispatch lacks task_id | extension host API gets nil task | automation cron task never advances | network delegation forwards wrong claim_token_hash (XCT-06) | events missing `task_id`/`run_id`/`claim_token_hash` | task APIs return inconsistent state | `agh task list` mismatches HTTP | web task pane stuck |
| **scheduler** | daemon shutdown joins forever | session never woken | task_runs starve | — | — | — | — | — | — | cron-fired sessions miss wake | — | wakeup events absent | scheduler endpoints lie | `agh scheduler status` lies | web idle indicator stuck |
| **hooks** | shutdown blocks waiting on hook drain | session start has no policy | claim path bypassed | — | — | memory provenance lost | skill activation policy bypassed | tool deny silently allowed (XCT-02) | extension hook surfaces incorrect | automation triggers no-op | network sends without policy | hook_decision events missing | api lacks deny telemetry | cli `agh observe events --type hook_decision` empty | web policy badge wrong |
| **memory** | shutdown leaks Lock gate | session prompt assembly missing recall | — | — | hook_writer can't persist provenance | — | skill memory tied to wrong scope | — | extension memory writes leak (XCT-07) | automation triggers from stale memory event | — | recall events absent | memory APIs inconsistent | `agh memory list` desyncs | web memory pane wrong |
| **skills** | watcher leaks | session sees stale skill list | — | — | hook decl divergence | memory recall references unknown skill | — | tool routing breaks for skill-bound tools | extension can't reload skills | automation has no skill catalog | — | skill events missing | skill APIs lie | `agh skill list` lies | web capability tile wrong (XCT-07) |
| **tools** | tool-process registry orphans | session prompt has no tools | — | — | hook can't narrow capabilities | — | skill effective tools wrong | — | extension tool dispatch fails | automation triggers tool runs that never finish | — | tool_call events missing | tool APIs inconsistent | `agh tool list` lies | web tool inspector wrong |
| **extension** | extension Stop never called → daemon leaks | session-create hook unstable | — | — | hooks fired but extension panics (XCT-11) | memory hook not invoked | skills/tools registered for dead extension | tool dispatch crashes daemon | — | automation can't use extension | network exposes broken extension | extension health events missing | extension APIs lie | `agh extension list` lies | web extensions panel wrong (XCT-07/XCT-11) |
| **automation** | cron entries lock store on shutdown | sessions started without lineage | claim path orphan | scheduler wakeups doubled | hooks see wrong actor_kind | — | — | — | — | — | — | automation events missing | automation APIs inconsistent | `agh automation list` lies | web automation pane wrong |
| **network** | shutdown leaves admitted wakes stuck | network-driven sessions orphan | claim_token_hash mismatched (XCT-06) | — | — | — | — | — | — | — | — | network events missing audit row (NET-19) | network APIs lie | `agh network status` lies | web network tile wrong |
| **observe** | shutdown blocks broadcaster | session SSE silent | task lineage hidden | — | hook telemetry hidden | memory consolidation events hidden | — | tool_call events hidden (TOL-08) | extension health invisible | automation telemetry hidden | network audit hidden | — | `/api/observe/*` lies | `agh observe events` empty | web event stream silent (XCT-08) |
| **api(http/uds)** | shutdown rejects refuse mid-shutdown wrong (DB-15) | session APIs unreachable | task APIs unreachable | scheduler unreachable | hook APIs unreachable | memory APIs unreachable | skills APIs unreachable | tools APIs unreachable | extension APIs unreachable | automation APIs unreachable | network APIs unreachable | observe APIs unreachable | — | cli loses transport | web loses transport |
| **cli** | — | session verbs lie | task verbs lie | — | — | memory verbs lie | skill verbs lie | tool verbs lie | extension verbs lie | automation verbs lie | network verbs lie | observe verbs lie | UDS verbs unreachable | — | — |
| **web** | — | session pane stale | — | — | — | memory pane stale | skills pane stale | tools pane stale | extensions pane stale | automation pane stale | network pane stale | observe pane stale | — | — | — |

**How to use this matrix during QA**: pick a release-blocker incident report; locate the misbehaving module (row); each non-empty cell is a sibling regression that the QA gate must additionally exercise. Example: if `task` silently misbehaves, network delegation (XCT-06) AND coordinator claim (XCT-05) AND tool dispatch correlation (XCT-02) AND web task pane (XCT-07 surface) must all be re-verified — not just `internal/task` unit tests.
