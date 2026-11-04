# Vocabulary alignment (Always-on) QA walk, #1199 S4

- Build: `feat/vocabulary-alignment-s4` at `1f207fc67`. This is the integration head: S1 backend, S2 web and S3 docs, rebased onto main.
- Lab: an isolated `eng-qa-bootstrap` targeted lab (`cli,api,web,runtime`) with its own `COMPOZY_HOME`, HTTP port and socket. The daemon was built from the worktree and served the built `web/dist`.
- Provider: the acpmock driver, routed through `[providers.claude] command`. The fixture is the one from `artifacts/2026-11-03-stays-on-hardening-qa/acpmock-fixture.json`, minus the turns pinned to that lab's IDs.
- Web screens were captured headless with Playwright at 1500×950. They stay local (lab `qa-artifacts/screens/`) and were not uploaded.
- Host restriction: partway through, the operator asked for a single process at a time with no further browser runs, because the host load was around 100. The scenarios not walked by then are marked `Deferred`. They were not judged pass or fail.

## Matrix

| Scenario | Verdict | Notes |
| --- | --- | --- |
| MS-web-agent-permissions | Partial | Five levels in order: Ask before every action, Ask before changes, Accept edits, Auto-review, Full access. Always-on with nothing set selects Accept edits with a Default pill. On-demand adds Follow Settings: "Uses your Settings. Right now: Full access." Settings → General shows the same five labels with Full access selected. Approval reach shows Only its own sessions / Any session it can reach. Saving Full access sent PUT 200, survived a reload, and `agent info -o json` gave `approve-all`. Not walked: the "has no AI reviewer" warning (the mock provider shows none), exceptions, extension read-only. |
| ET-web-agent-detail-tab-parity | Partial | With a project bound (`?workspace=`), Instructions (`/what-it-does`), Heartbeat (`/check-ins`, "Next heartbeat"), Memory and Where it works open with the right titles. Unbound under All projects, Instructions and Heartbeat show "Couldn't load this page" (defect 1, known since 2026-11-01). Digest conflict and the unsaved guard were not walked. |
| MS-web-agent-create-simple-advanced | Partial | One screen. How it works offers "On-demand: runs when you ask" / "Always-on: watches for you". Always-on adds Where it works (Global) and "Create and start". Advanced holds the permission cards. Create with Full access redirected to Sessions and placed the avatar. Taken name, discard prompt and Fill with AI were not walked. |
| MS-web-stays-on-placement | Partial | The toolbar is labeled "Always-on agents" in List and Kanban. Avatars read "<Agent> · Global/vocablab · Idle". The hover card shows "Last heartbeat · next in 25m" and Open conversation. The context menu shows Pause, Open agent, Heartbeat, Archive. An agent edited to `on_demand` on disk paused with `definition_not_always_on`. Its hover and menu read "Paused because this agent is no longer always-on.", Resume is disabled, and the home reads "It can't resume" / "Now on-demand, so it can't be always-on". Not walked: `+N`, needs-you ordering. |
| MS-web-stays-on-reports | Deferred | Host overload restriction. |
| ET-web-agents-team-profile | Partial | `/agents` shows the Always-on section (7) above On-demand (5) in Cards. Agents loaded from `kind: stays_on` files (scout, pscout, wswatch) are listed as Always-on. Home Setup rows read Instructions, Model, Heartbeat, Memory, Tools, Permissions for always-on, and Instructions, Model, Tools, Permissions for on-demand. The Model row is the selector. Rows view and old-link redirects were not walked. |
| ET-web-agent-fleet-listing-rows | Deferred | Host overload restriction. |
| RT-session-monitor-wake | Deferred | Host overload restriction. |
| RT-agent-memory-startup | Partial | CLI only. `agent memory add scout …` saved. The on-demand `helper` returns `{"error":"agent_not_always_on"}`. A new always-on main session's first turn carries the `always_on_doctrine` and `agent_memory` startup spans, and no `stays_on_doctrine`. On-demand session startup and note retention after edits were not walked. |
| TA-web-automation-editor | Deferred | Host overload restriction. |
| TA-orchestrator-small-work | Blocked | The scenario requires a real provider (worktree dispatch and evidence). The lab is provider-free. |

## Upgrade seed (US-004, US-005)

Seeded before the first boot: global `agents/scout`, profile `profiles/default/agents/pscout` and workspace `.compozy/agents/wswatch`, all `kind: stays_on`, plus `[agents.stays_on] approval_timeout = "2h"`.

- Boot 1 logged `INFO rewrote agent kind stays_on to always_on` once for the global file and once for the profile file. The workspace file was not rewritten.
- `cmp` against the seed copies, with only `kind: stays_on` replaced by `kind: always_on`, is identical for both rewritten files. Mode stays 0644. The workspace file is byte-identical to its seed.
- The workspace file loads with one `WARN agent kind stays_on is deprecated; use always_on (stays_on is removed in v0.5.2) path=…/wswatch/AGENT.md` per daemon process.
- The daemon log has one `WARN agents.stays_on is deprecated; use agents.always_on (removed in v0.5.2) keys=approval_timeout`.
- `compozy config get agents.always_on.approval_timeout` prints `agents.always_on.approval_timeout: 2h0m0s`. Every duration key prints in this Go format, for example `triage_timeout: 10m0s`.
- `config get agents.stays_on.approval_timeout` warns on stderr and returns the new path.
- Boots 2 and 3 had no rewrite line.
- All three seeded agents report `kind: always_on`.

## CLI Golden Path (E2E-003) and API

- `agent create triage --kind always_on --provider claude --prompt …` → existing Agent section output with `Kind: always_on`, exit 0.
- `agent instance create triage --workspace vocablab` → `Triage is always-on in vocablab` / `Main session: sess-…`.
- `agent instance create triage2 --global` → `Triage2 is always-on in Global`.
- `agent info triage -o json | jq .kind` → `"always_on"`.
- `agent create triage2 --kind stays_on …` → stderr `warning: --kind stays_on is deprecated; use always_on (removed in v0.5.2)`, created always-on.
- `agent update triage2 --kind stays_on --expected-digest …` → the same warning, and the update proceeds. The file says `kind: always_on`.
- `agent update triage --kind on_demand` → `agent_kind_immutable: an agent's kind is fixed at creation; duplicate it to choose another kind`, exit 1.
- `agent create triage3 --kind sometimes` → "agent kind must be on_demand or always_on", exit 65. See defect 3.
- `agent instance create helper` (on-demand) → "This agent is on-demand. Only always-on agents have instances.", exit 78. With `-o json`, the code is `agent_not_always_on`.
- `POST /api/agents` with `"kind":"stays_on"` → 201 and `kind: always_on`. The daemon logs one `WARN … source=request agent=triagehttp`.
- `POST /api/agents` with `"kind":"sometimes"` → 400 `agent_kind_invalid`.
- Help text: `--kind` reads "Agent kind: on_demand (default) or always_on; fixed at creation", and `agent instance` reads "Manage always-on agent instances".

## Product language

`bash scripts/check-product-language.sh` and `make product-language-check` at `1f207fc67` both exit 0.

## Findings

1. **Re-found, pre-existing: Instructions and Heartbeat pages fail unbound under All projects.**
   - Reproduction: All projects → Agents → open a Global-only agent (`qa-watcher`), or the project-placed `scout` by URL → click the Heartbeat row.
   - Expected: the page body. Actual: "Couldn't load this page".
   - Cause: `GET /api/agents/{name}/soul` and `/heartbeat` return 400 "workspace_id is required" (`internal/api/core/authored_context_target.go:57-59`).
   - The same pages load with `?workspace=`. This was recorded as open defect 1 in `2026-11-01-stays-on-agents.md`, which needs a storage decision. This walk adds that a project-placed agent opened by URL also fails.
   - Not caused by #1199; the branch only renames copy in these files.
2. **Copy (Polish, S2): visible "check in" verbs remain beside "Heartbeat".** ADR-003 makes "heartbeat" the UI word for a scheduled run. These strings still use the old verb:
   - "It checked in very recently, so it waited." and "It had no chat to check in from." (`web/src/systems/agent/lib/agent-check-in-activity.ts:37,38,48`)
   - "Nowhere to check in yet" (`web/src/systems/agent/components/agent-editors/check-ins-view.tsx:152`)
   - "Last checked in never", seen on the paused agent home (`web/src/systems/agent/lib/agent-profile.ts:79`)
3. **Contract text vs existing CLI behavior: CLI error rows.**
   - `_dx.md` §CLI lists exit 1 with a `agent_kind_invalid:` / `agent_not_always_on:` prefix.
   - The CLI prints the daemon message without the code and maps HTTP 400 → 65 and 422 → 78 (`internal/cli/client_api_errors.go:238-252`, unchanged by this branch). `-o json` carries the code.
   - Recommendation: correct the `_dx.md` rows, not the generic mapping.

Paper cuts:

- Every CLI command prints the config deprecation WARN on stderr while `[agents.stays_on]` is present. This is expected behavior: once per process.
- The Settings header reads "1 active sessions · 1 agents working", and is pre-existing.

## Follow-up

Shim removal is tracked in #1239 (milestone v0.5.2).
