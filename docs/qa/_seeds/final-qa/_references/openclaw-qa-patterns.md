---
name: openclaw-qa-patterns
description: openclaw QA framework reference — scenario template, qa-channel, frontier harness, live-vs-mock policy, qa coverage CLI, evidence rules. Adopted as inspiration for AGH final-qa.
type: reference
source: .resources/openclaw/
---

# openclaw QA Framework — Reference Patterns

## 1. Operating Model (the philosophy)

openclaw splits the QA stack into three concentric concerns: a synthetic
transport (`qa-channel`) that lets the harness drive any agent over a
deterministic Slack-class bus; a generic markdown scenario runner (`qa-lab`)
that owns gateway lifecycle, worker concurrency, artifact writing and
reporting; and per-transport runner adapters (Matrix, Telegram, Discord,
Multipass) that mount under the same `openclaw qa <runner>` root.

> "The private QA stack is meant to exercise OpenClaw in a more realistic,
> channel-shaped way than a single unit test can."
>
> source: `.resources/openclaw/docs/concepts/qa-e2e-automation.md` (line 11)

QA is treated as an operator-shaped workflow, not as a test suite. Every run
has an authored "QA mission" delivered to an in-character QA agent
(`Dev C-3PO`) that reads the repo before acting, executes scenarios, and
ends with a Worked / Failed / Blocked / Follow-up protocol report.

> "Run the scenarios through the real qa-channel surfaces where possible.
> Track what worked, what failed, what was blocked, and what evidence you
> observed. End with a concise report grouped into worked / failed / blocked
> / follow-up."
>
> source: `.resources/openclaw/qa/scenarios/index.md` (lines 67-77)

**Vocabulary used by the framework**:

- **qa suite** — the executable repo-backed regression / frontier subset.
  Run with `pnpm openclaw qa suite`. Aliases for runners under one root:
  `qa suite --runner multipass`. (`docs/concepts/qa-e2e-automation.md` table
  at lines 30-47.)
- **qa manual** — a one-off prompt against a chosen provider/model, used as a
  scoped personality/style probe after the executable subset is green.
- **qa coverage** — coverage inventory CLI; reads `coverage.primary` and
  `coverage.secondary` IDs from each scenario's `qa-scenario` block and
  prints a markdown/JSON inventory.
- **qa parity-report** — compares two `qa-suite-summary.json` artifacts and
  emits an "agentic parity" verdict that gates the candidate model against a
  baseline.
- **qa character-eval** — runs the same scenario against many live model refs
  and asks judge models to rank naturalness, vibe and humor (blind-judge
  optional).
- **qa-lab** — the dashboard UI plus QA bus extension that observes the
  transcript, injects messages and exports the markdown report.
- **qa-channel** — the bundled synthetic message channel plugin. Slack-class
  target grammar `dm:<user>`, `channel:<room>`, `thread:<room>/<thread>`;
  HTTP-backed bus for inbound injection, outbound transcript capture, thread
  creation, reactions, edits, deletes, and search/read actions
  (`docs/channels/qa-channel.md` lines 13-19).
- **frontier harness** — a small "tune the harness on big models first"
  subset that gets run before the wider seed suite when changing the prompt
  or harness (`qa/frontier-harness-plan.md` lines 5-31).
- **bakeoff loop** — the GPT → Claude → Gemini sweep order used while tuning
  the harness, with provider-specific overlays preferred over shared-prompt
  rewrites when only one family regresses (`qa/frontier-harness-plan.md`
  lines 26-31, 75-81).
- **convex credential broker** — standalone Convex v1 lease broker that pools
  live transport credentials so multiple maintainers / CI runs can share one
  Telegram or Discord setup without colliding (`qa/convex-credential-broker/
  README.md`).
- **provider mode** — `live-frontier` (real provider keys), `mock-openai`
  (scenario-aware deterministic dispatcher), or `aimock` (additive AIMock
  protocol/fixture lane) (`docs/concepts/qa-e2e-automation.md` lines
  304-318).

**Operator identity**. The QA agent has a persona/style block burned into
`qa/scenarios/index.md` that ships with the seed pack. The kickoff task tells
the agent to read source/docs first, run the seeded scenarios on real
qa-channel surfaces, propose extra scenarios, and end every run with a
worked/failed/blocked/follow-up report. Every run inherits this identity by
default unless a scenario overrides it.

> "Persona: protocol-minded, precise, a little flustered, conscientious,
> eager to report what worked, failed, or remains blocked. Style: read source
> and docs first, test systematically, record evidence, end with a concise
> protocol report."
>
> source: `.resources/openclaw/qa/scenarios/index.md` (lines 47-60)

## 2. Scenario File Anatomy

Every scenario lives in `qa/scenarios/<theme>/<slug>.md` and contains two
fenced blocks: a YAML frontmatter `qa-scenario` (declarative metadata) and a
YAML `qa-flow` (imperative execution steps).

**Canonical `qa-scenario` block fields** (from
`.resources/openclaw/extensions/qa-lab/src/scenario-catalog.ts` lines
174-193, the Zod schema):

| Field                 | Description                                                                                                                                                | Example                                                                              |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| `id`                  | Stable scenario id used by `--scenario` and reports. Lowercase, dotted/dashed.                                                                             | `memory-recall`                                                                      |
| `title`               | Human-readable scenario title.                                                                                                                             | `"Memory recall after context switch"`                                               |
| `surface`             | Required single-word surface tag (used in coverage `bySurface` index).                                                                                     | `memory`                                                                             |
| `surfaces`            | Optional array overriding `surface` when one scenario covers multiple surfaces.                                                                            | `[memory, channels.qa-channel]`                                                      |
| `category`            | Optional category metadata.                                                                                                                                | `runtime`                                                                            |
| `coverage.primary`    | Required behavior IDs that this scenario primarily protects. Lowercase dotted/dashed tokens, regex `^[a-z0-9]+(?:[.-][a-z0-9]+)*$`. Min length 1.          | `[memory.recall]`                                                                    |
| `coverage.secondary`  | Optional behavior IDs the scenario also protects.                                                                                                          | `[channels.qa-channel]`                                                              |
| `risk` / `riskLevel`  | Optional `low` / `medium` / `high` severity (or free-form `riskLevel`).                                                                                    | `medium`                                                                             |
| `capabilities`        | Optional capability tags.                                                                                                                                  | `["tools.read","tools.write"]`                                                       |
| `lane`                | Optional record (string→bool/string) for lane filters (e.g. mock-only / live-only flags).                                                                  | `{mock: true, live: false}`                                                          |
| `objective`           | Required one-sentence objective. Used by the report.                                                                                                       | `"Verify the agent can store a fact, switch topics, then recall the fact later."`    |
| `successCriteria`     | Required min-1 array of plain-language pass conditions. Used by the markdown report and by the agent in self-evaluation.                                   | `["Agent acknowledges the seeded fact.", "Agent later recalls the same fact."]`      |
| `plugins`             | Optional list of bundled plugins the scenario needs activated.                                                                                             | `["diagnostics-otel"]`                                                               |
| `gatewayConfigPatch`  | Optional JSON-Patch-shaped overrides applied to the child gateway before the scenario runs.                                                                | `{diagnostics: {enabled: true, otel: {traces: true}}}`                               |
| `gatewayRuntime`      | Optional gateway runtime tweaks (e.g. `forwardHostHome: true` to let scenario use the real host home — used by Control UI image roundtrip).                | `{forwardHostHome: true}`                                                            |
| `docsRefs`            | Required-for-discoverability array of repo-relative doc paths the scenario protects. Drives report traceability.                                           | `["docs/concepts/memory.md","docs/concepts/memory-search.md"]`                       |
| `codeRefs`            | Repo-relative source paths the scenario exercises.                                                                                                         | `["extensions/memory-core/src/tools.ts"]`                                            |
| `execution.kind`      | `flow` (the only supported kind today). Defaults to `flow`.                                                                                                | `flow`                                                                               |
| `execution.summary`   | One-sentence description of what the flow does — different from `objective` in that it is implementation-shaped instead of behavior-shaped.                | `"Verify the agent can store a fact, switch topics, then recall the fact later."`   |
| `execution.config`    | Free-form `Record<string,unknown>` of scenario-specific knobs. Convention: keys ending in `Any` MUST be `string[]` (validated by Zod `superRefine`).       | `{rememberPrompt: "...", recallExpectedAny: ["alpha-7"]}`                            |

The `qa-pack` block in `qa/scenarios/index.md` adds two pack-level fields:
`agent.identityMarkdown` (the operator identity) and `kickoffTask` (the
mission body).

**Canonical `qa-flow` block sections** (from
`extensions/qa-lab/src/scenario-catalog.ts` lines 96-172):

- `steps[]` — at least one step. Each step has:
  - `name` — required short label.
  - `actions[]` — at least one action. Action kinds:
    - `call: <fn>` (with `args` array and optional `saveAs`) — invokes one of
      the named runtime helpers (`reset`, `runAgentPrompt`,
      `waitForOutboundMessage`, `state.addInboundMessage`, `fs.writeFile`,
      `fs.readFile`, `env.gateway.call`, `injectInboundMessage`, etc.).
    - `set: <var>` with `value: …` (literal, `expr:` or `ref:`).
    - `assert: <expr>` (or `{expr, message}`) — boolean expression evaluated
      against the scenario scope; failure is a step failure.
    - `throw: …` — explicit failure with formatted message.
    - `if: {expr, then, else?}` — branching.
    - `forEach: {items, item, index?, actions}` — loop.
    - `try: {actions, catchAs?, catch?, finally?}` — recovery / guarded
      sections.
  - `detailsExpr` — optional expression whose stringified value lands in the
    final report's "details" column for human review.

**Helper grammar surface** (preferred names per `docs/concepts/qa-e2e-automation.md`
lines 376-393): `waitForTransportReady`, `waitForChannelReady`,
`injectInboundMessage`, `injectOutboundMessage`, `waitForTransportOutboundMessage`,
`waitForChannelOutboundMessage`, `waitForNoTransportOutbound`,
`getTransportSnapshot`, `readTransportMessage`, `readTransportTranscript`,
`formatTransportTranscript`, `resetTransport`. Compatibility aliases:
`waitForQaChannelReady`, `waitForOutboundMessage`, `waitForNoOutbound`,
`formatConversationTranscript`, `resetBus`.

**Complete scenario example** (`qa/scenarios/scheduling/cron-single-run-no-duplicate.md`,
chosen because it exercises both metadata and a representative
`assert + duplicate-window` pattern):

````markdown
# Cron single run no duplicate

```yaml qa-scenario
id: cron-single-run-no-duplicate
title: Cron single run no duplicate
surface: cron
coverage:
  primary:
    - scheduling.cron
  secondary:
    - channels.qa-channel
    - scheduling.dedup
objective: Verify one forced cron run produces exactly one qa-channel delivery for its marker.
successCriteria:
  - A single forced cron run completes successfully.
  - The qa-channel receives exactly one outbound reply containing the run marker.
  - No second outbound reply with the same marker appears during the duplicate window.
docsRefs:
  - docs/help/testing.md
  - docs/channels/qa-channel.md
codeRefs:
  - src/cron/service.ts
  - src/cron/run-log.ts
  - extensions/qa-lab/src/cron-run-wait.ts
  - extensions/qa-lab/src/suite-runtime-transport.ts
execution:
  kind: flow
  summary: Force one cron run and assert qa-channel does not receive a duplicate delivery for the same marker.
  config:
    channelId: qa-room
    channelTitle: QA Room
    duplicateWindowMs: 8000
    reminderPromptTemplate: "A QA cron dedupe check fired. Send a one-line ping back to the room containing this exact marker: {{marker}}"
```

```yaml qa-flow
steps:
  - name: creates a future cron job and forces one run
    actions:
      - call: reset
      - set: scheduledFor
        value:
          expr: "new Date(Date.now() + 10 * 60 * 1000).toISOString()"
      - set: cronMarker
        value:
          expr: "`QA-CRON-DEDUPE-${randomUUID().slice(0, 8)}`"
      - call: env.gateway.call
        saveAs: response
        args:
          - cron.add
          - name:
              expr: "`qa-dedupe-${randomUUID()}`"
            enabled: true
            schedule:
              kind: at
              at:
                ref: scheduledFor
            sessionTarget: isolated
            wakeMode: now
            payload:
              kind: agentTurn
              message:
                expr: "config.reminderPromptTemplate.replace('{{marker}}', cronMarker)"
            delivery:
              mode: announce
              channel: qa-channel
              to:
                expr: "`channel:${config.channelId}`"
      - set: jobId
        value:
          expr: response.id
      - call: env.gateway.call
        saveAs: runResponse
        args:
          - cron.run
          - id: { ref: jobId }
            mode: force
          - timeoutMs: 30000
      - assert:
          expr: "runResponse?.ok === true && runResponse?.ran !== false"
          message:
            expr: "`expected cron.run to enqueue one run, got ${JSON.stringify(runResponse)}`"
    detailsExpr: "`job=${jobId} marker=${cronMarker}`"

  - name: observes exactly one qa-channel delivery for that run
    actions:
      - call: waitForCronRunCompletion
      - call: waitForOutboundMessage
        saveAs: firstOutbound
      - call: sleep
        args:
          - expr: config.duplicateWindowMs
      - set: duplicateMatches
        value:
          expr: "getTransportSnapshot().messages.filter((message) => message.direction === 'outbound' && message.conversation.id === config.channelId && message.text.includes(cronMarker))"
      - assert:
          expr: "duplicateMatches.length === 1"
          message:
            expr: "`expected one outbound delivery for ${cronMarker}, saw ${duplicateMatches.length}`"
```
````

(Source: `.resources/openclaw/qa/scenarios/scheduling/cron-single-run-no-duplicate.md`,
edited only by trimming a few `args` to reduce noise — every key shown
exists verbatim in the source file.)

## 3. Themed Coverage Map

Themes are defined in `qa/scenarios/index.md` (lines 27-39). Coverage IDs
listed in the table below come directly from each scenario's `coverage.primary`
field. This is what openclaw considers MUST-cover surface area.

| Theme        | # scenarios | Purpose                                                                                                            | Example titles                                                                                                                                                                                                                                                                                                                                       |
| ------------ | ----------- | ------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `agents`     | 6           | Agent instructions, subagent fan-out / handoff / forked-context / direct fallback / stale child-link regressions.  | `instruction-followthrough-repo-contract`, `subagent-completion-direct-fallback`, `subagent-fanout-synthesis`, `subagent-forked-context`, `subagent-handoff`, `subagent-stale-child-links`                                                                                                                                                          |
| `channels`   | 5           | DM, channel, thread, message-action, qa-channel reconnect/dedupe.                                                  | `channel-chat-baseline`, `dm-chat-baseline`, `qa-channel-reconnect-dedupe`, `reaction-edit-delete`, `thread-follow-up`                                                                                                                                                                                                                              |
| `character`  | 2           | Persona / style probes for later judge grading. Forbidden-needle lists keep transport errors out of the transcript.| `character-vibes-c3po`, `character-vibes-gollum`                                                                                                                                                                                                                                                                                                    |
| `config`     | 4           | `config.patch` hot apply + skill toggle, `config.apply` restart wake-up, capability flip, ring-zero setup.         | `config-apply-restart-wakeup`, `config-patch-hot-apply`, `config-restart-capability-flip`, `crestodian-ring-zero-setup`                                                                                                                                                                                                                              |
| `media`      | 3           | Image understanding from attachment, image generation roundtrip, native `image_generate` tool gating.              | `image-generation-roundtrip`, `image-understanding-attachment`, `native-image-generation`                                                                                                                                                                                                                                                            |
| `memory`     | 7           | Recall, ranking, active memory, dreaming sweep, memory failure fallback, on-demand memory tools, thread isolation. | `active-memory-preprompt-recall`, `memory-dreaming-sweep`, `memory-failure-fallback`, `memory-recall`, `memory-tools-channel-context`, `session-memory-ranking`, `thread-memory-isolation`                                                                                                                                                          |
| `models`     | 10          | Provider auth/setup-token, model switching, thinking-mode visibility, codex meta-leak, openai web search.          | `anthropic-opus-api-key-smoke`, `anthropic-opus-setup-token-smoke`, `claude-cli-provider-capabilities`, `claude-cli-provider-capabilities-subscription`, `codex-harness-no-meta-leak`, `gpt55-thinking-visibility-switch`, `model-switch-follow-up`, `model-switch-tool-continuity`, `openai-native-web-search-live`, `thinking-slash-model-remap`    |
| `plugins`    | 8           | Bundled plugin runtime, MCP plugin tools call, plugin lifecycle hot reload, skill install/visibility, skill workshop. | `bundled-plugin-skill-runtime`, `mcp-plugin-tools-call`, `plugin-lifecycle-hot-reload`, `skill-install-hot-availability`, `skill-visibility-invocation`, `skill-workshop-animated-gif-autocreate`, `skill-workshop-pending-approval`, `skill-workshop-reviewer-autonomous`                                                                          |
| `runtime`    | 11          | Approval followthrough, compaction retry under mutating writes, OTEL/Prometheus, gateway-restart in-flight, streaming integrity, empty-response recovery, reasoning-only retry policy, inventory drift. | `approval-turn-tool-followthrough`, `compaction-retry-mutating-tool`, `docker-prometheus-smoke`, `empty-response-recovery-replay-safe-read`, `empty-response-retry-budget-exhausted`, `gateway-restart-inflight-run`, `otel-trace-smoke`, `reasoning-only-no-auto-retry-after-write`, `reasoning-only-recovery-replay-safe-read`, `runtime-inventory-drift-check`, `streaming-final-integrity` |
| `scheduling` | 3           | Cron one-minute ping, single-run-no-duplicate, natural-fire-no-duplicate.                                          | `cron-natural-fire-no-duplicate`, `cron-one-minute-ping`, `cron-single-run-no-duplicate`                                                                                                                                                                                                                                                            |
| `security`   | 1           | Fake-secret redaction in tool/log output (no leak into outbound channel transcript).                               | `secret-redaction-tool-logs`                                                                                                                                                                                                                                                                                                                         |
| `ui`         | 1           | Embedded Control UI driving qa-channel session through `web*` browser helpers (text + image roundtrip + transcript persistence on fresh page load). | `control-ui-qa-channel-image-roundtrip`                                                                                                                                                                                                                                                                                  |
| `workspace`  | 5           | Lobster Invaders build artifact, long-running release audit, medium-game-plan codex/pi harness, source/docs discovery report. | `lobster-invaders-build`, `long-running-release-audit`, `medium-game-plan-codex-harness`, `medium-game-plan-pi-harness`, `source-docs-discovery-report`                                                                                                                                                                  |

Total: 66 scenarios across 13 themes.

## 4. Real-LLM vs Mock Policy

openclaw runs three provider modes against the same scenario file:

> "`qa suite` has two local provider mock lanes: `mock-openai` is the
> scenario-aware OpenClaw mock. It remains the default deterministic mock
> lane for repo-backed QA and parity gates. `aimock` starts an AIMock-backed
> provider server for experimental protocol, fixture, record/replay, and
> chaos coverage. It is additive and does not replace the `mock-openai`
> scenario dispatcher."
>
> source: `.resources/openclaw/docs/concepts/qa-e2e-automation.md` (lines 304-318)

- `mock-openai` — the scenario-aware deterministic dispatcher. Records every
  request to `${env.mock.baseUrl}/debug/requests` so scenarios can assert
  `plannedToolName === 'read'` etc.; this is the **mandatory** pre-tool
  evidence that scenarios use to prove the model actually called the right
  tool, not just produced plausible prose. (See criterion 2 of the parity
  completion gate, e.g. `qa/scenarios/agents/subagent-handoff.md` lines 52-72:
  "require an actual `sessions_spawn` tool call. Without this, a model could
  produce the three labeled sections … as free-form prose without ever
  delegating to a real subagent.")
- `live-frontier` — real provider keys. Default for transport lanes
  (Matrix/Telegram/Discord) and for the frontier harness sweeps.
- `aimock` — additive, opt-in for protocol-shape work; not a replacement.

**When a scenario mandates live**: scenarios assert
`env.providerMode === 'live-frontier'` (or `requiredProvider`) at flow start
and skip / fail otherwise. Example: `anthropic-opus-api-key-smoke.md` lines
36-53 fail fast if the live primary provider is not Anthropic, the model is
not `claude-opus-4-8`, or `ANTHROPIC_API_KEY` is missing.

**When mocks are mandated**: scenarios that test runtime retry/recovery
behavior (e.g. `empty-response-recovery-replay-safe-read.md`) gate on
`env.providerMode === 'mock-openai'` because the deterministic dispatcher is
the only way to force the empty-response path.

> "this seeded scenario is mock-openai only"
>
> source: `.resources/openclaw/qa/scenarios/runtime/empty-response-recovery-replay-safe-read.md`
> (line 39)

**Live transport coverage matrix** — three live-transport lanes share one
explicit checklist (`docs/concepts/qa-e2e-automation.md` lines 127-139). Each
lane writes:

- A `*-qa-report.md` markdown protocol report (Worked / Failed / Blocked /
  Follow-up).
- A `*-qa-summary.json` machine-readable summary; replying scenarios include
  per-reply RTT (driver send → observed SUT reply).
- A `*-qa-observed-{messages,events}.json` artifact. Bodies are redacted by
  default; opt-in capture flags (e.g. `OPENCLAW_QA_TELEGRAM_CAPTURE_CONTENT=1`)
  keep bodies for bug reports.
- A combined `*-qa-output.log` stdout/stderr.

> "Bodies are redacted unless `OPENCLAW_QA_MATRIX_CAPTURE_CONTENT=1`;
> approval metadata is summarized with selected safe fields and truncated
> command preview."
>
> source: `.resources/openclaw/docs/concepts/qa-matrix.md` (line 116)

**The Convex credential broker** is openclaw's solution to "many maintainers
plus CI all need the same Telegram bot pair". It is a standalone Convex v1
project under `qa/convex-credential-broker/` with six HTTP endpoints
(`/acquire`, `/heartbeat`, `/release`, `/admin/{add,remove,list}`). Pool
partitioning is by `kind` only (`telegram`, `discord`); selection is
least-recently-leased; callers fail fast when the broker is unreachable;
secrets are split into maintainer / CI roles
(`OPENCLAW_QA_CONVEX_SECRET_MAINTAINER` vs `_CI`). On admin/add the broker
validates the payload shape per kind (e.g. for telegram: `{groupId,
driverToken, sutToken}` with numeric chat-id string).

> "QA lab acquires an exclusive lease, heartbeats it for the duration of the
> run, and releases it on shutdown. Pool kinds are `"telegram"` and
> `"discord"`."
>
> source: `.resources/openclaw/docs/concepts/qa-e2e-automation.md` (line 253)

Why pooled live creds matter at scale: they let a release PR run all three
live transport lanes in parallel without bot-account collision; they let CI
acquire a lease with role `ci` while a maintainer holds another with role
`maintainer`; and they keep the credential set out of every contributor's
shell profile.

## 5. qa-channel — Control + Image Roundtrip

`qa-channel` is the bundled synthetic Slack-class transport plugin used by
`qa-lab` for repo-backed scenarios. Scenarios drive the channel through the
`qa-lab` API surface (e.g. `state.addInboundMessage`, `injectInboundMessage`
with `attachments[]`, `handleQaAction` for thread create / reaction / edit /
delete, `getTransportSnapshot` for transcript inspection).

**Wiring**: configured under `channels.qa-channel.{baseUrl, botUserId,
botDisplayName, allowFrom, pollTimeoutMs, defaultTo,
actions.{messages,reactions,search,threads}}`
(`docs/channels/qa-channel.md` lines 22-49). Multi-account keys at the top
level (`accounts`, `defaultAccount`) let one runner serve several
synthetic identities.

**Control vs image messages**: the channel exposes the same payload grammar
as a real transport. Inbound injection accepts a `text` plus an
`attachments[]` array with `kind`, `mimeType`, `fileName`, `altText`,
`contentBase64` — see `qa/scenarios/ui/control-ui-qa-channel-image-roundtrip.md`
lines 211-229 (image PNG injected through `injectInboundMessage`) and
`qa/scenarios/media/image-understanding-attachment.md` lines 48-53 (image
PNG injected as `attachments` to `runAgentPrompt`).

**Why this is interesting for AGH**:

1. **Programmatic test signals** — the same channel surface that a user types
   into is the surface a scenario injects into; there is no parallel "test
   API" surface that drifts away from production behavior.
2. **Evidence capture for free** — the synthetic bus records every inbound
   and outbound message into `state.getSnapshot().messages`, including
   `direction`, `conversation.{id,kind,title}`, `threadId`, `text`, and
   `attachments`. Scenarios assert against this snapshot directly
   (`firstMatchesBeforeFollowup.length === 1`, `markerOutbounds.length === 1`,
   `tokenDeltaLike.length === 0` etc.).
3. **Round-tripping the Control UI** — the `control-ui-qa-channel-image-roundtrip`
   scenario opens the embedded Control UI through the gateway's `browser.request`
   seam (`webOpenPage`, `webWait`, `webSnapshot`, `webEvaluate`) and asserts
   that text and image messages injected through qa-channel land in the
   transcript on a *fresh page load* (transcript persistence proof). This
   doubles as a Web QA harness without standing up Playwright separately
   (`qa/scenarios/ui/control-ui-qa-channel-image-roundtrip.md` lines 158-296).

## 6. Frontier Harness + Bakeoff Loop

The "frontier harness" is a small subset of scenarios run *before* anyone
touches the seed suite. The plan lives in
`.resources/openclaw/qa/frontier-harness-plan.md`.

**Frontier subset** (`qa/frontier-harness-plan.md` lines 14-25):

- `approval-turn-tool-followthrough` — short approval ("ok do it") triggers
  a real tool call, not fake-progress narration.
- `model-switch-tool-continuity` — model swap mid-session keeps tool use,
  not just plain text.
- `source-docs-discovery-report` — agent reads docs and source before
  proposing extra tests, ends with worked/failed/blocked/follow-up.

**Longer spot-check after that**: `compaction-retry-mutating-tool`,
`subagent-handoff`.

**Bakeoff loop / baseline order**:

1. GPT first (main tuning reference: `openai/gpt-5.5`).
2. Claude second. If Claude regresses alone, prefer an Anthropic overlay
   fix over a core prompt rewrite.
3. Gemini third (operational-directness check).
4. Only run the whole seed suite after the frontier subset is stable.

**Tuning loop**:

> "1. Run the GPT subset and save the report path. 2. Patch one harness
> idea at a time. 3. Rerun the same GPT subset immediately. 4. If GPT
> improves, run the Claude subset. 5. If Claude is clean, run the Gemini
> subset. 6. If only one family regresses, fix the provider overlay before
> touching the shared prompt again."
>
> source: `.resources/openclaw/qa/frontier-harness-plan.md` (lines 75-81)

**What gets scored** (lines 83-92): tool commitment after `ok do it`;
empty-promise rate; tool continuity after model switch; discovery report
completeness and specificity; replay-safety truth after a mutating write;
scope drift (unrelated scenario updates, grand wrap-ups, invented completion
tallies); latency / obvious stall behavior; token cost notes if the change
makes the prompt heavier.

**Manual personality lane** runs *after* the executable subset, never
before, with `qa manual --message "<prompt>"` against each model
(`qa/frontier-harness-plan.md` lines 96-128).

**Regression-vs-frontier separation**. The seed scenario suite is the
regression loop (`qa suite`). The frontier subset above is what gates
prompt/harness changes before they ever touch regression. The
`qa parity-report` command compares two `qa-suite-summary.json` artifacts
(candidate vs baseline) and emits an "agentic parity" verdict; the report
hard-fails on missing required parity scenarios and on parity scenarios that
ran on both sides but failed (`extensions/qa-lab/src/agentic-parity-report.ts`
lines 405-430).

## 7. qa Coverage / Inventory Tooling

`pnpm openclaw qa coverage [--json]` prints a markdown / JSON inventory
derived entirely from scenario frontmatter. Implementation:
`.resources/openclaw/extensions/qa-lab/src/coverage-report.ts` (function
`buildQaCoverageInventory`).

The inventory exposes:

- `scenarioCount` — total scenarios.
- `coverageIdCount` — total distinct primary+secondary coverage IDs.
- `primaryCoverageIdCount` / `secondaryCoverageIdCount` — split.
- `features[]` — per-coverage-id list of `{id, scenarios[]}`. Each scenario
  reference carries `intent: "primary" | "secondary"`, `theme`, `surfaces[]`,
  `risk`.
- `overlappingCoverage[]` — coverage IDs touched by more than one scenario.
- `missingCoverage[]` — scenarios with no coverage block (a discoverability
  flag).
- `byTheme` / `bySurface` — same features re-grouped.

Coverage drift detection is implicit: the schema rejects:

- duplicate IDs inside one scenario's primary+secondary list (Zod
  `superRefine` at `scenario-catalog.ts` lines 67-89);
- non-conforming IDs (regex `^[a-z0-9]+(?:[.-][a-z0-9]+)*$` —
  `scenario-catalog.ts` lines 54-59);
- the older `coverage: ["id"]` flat-list shape (per
  `qa/scenarios/index.md` line 23, "treat the old `coverage: ["id"]` /
  `coverage: - id` list shape as invalid").

Scenario IDs are kept stable when files move; `docsRefs` and `codeRefs` carry
implementation traceability instead.

> "Source-path tracking [is] in the report, not in the scenario schema."
>
> source: `.resources/openclaw/qa/scenarios/index.md` (line 24)

## 8. SDK Testing & Plugin Testing

For plugin authors and bundled-plugin maintainers, openclaw ships a
focused-subpath test SDK at `openclaw/plugin-sdk/*` instead of one
monolithic `testing` barrel. The shape is documented in
`.resources/openclaw/docs/plugins/sdk-testing.md`.

Key utilities:

- `plugin-sdk/plugin-test-api` — `createTestPluginApi(...)` for direct
  registration unit tests against a minimal plugin API mock.
- `plugin-sdk/plugin-test-runtime` — `registerSingleProviderPlugin`,
  `registerProviderPlugin(s)`, `createRuntimeEnv`, `createTestRegistry`,
  `setActivePluginRegistry`. Used to drive loader-backed smoke tests so unit
  tests don't pretend the plugin would load.
- `plugin-sdk/channel-contract-testing` —
  `expectChannelInboundContextContract`,
  `installChannelOutboundPayloadContractSuite`. Reusable contract suites
  iterating over every registered channel.
- `plugin-sdk/channel-test-helpers` — `createStartAccountContext`,
  `installChannelActionsContractSuite`, `installChannelSetupContractSuite`,
  `installChannelStatusContractSuite`, `formatEnvelopeTimestamp`,
  `assertBundledChannelEntries`.
- `plugin-sdk/agent-runtime-test-contracts` — fixtures
  `AUTH_PROFILE_RUNTIME_CONTRACT`, `DELIVERY_NO_REPLY_RUNTIME_CONTRACT`,
  `OUTCOME_FALLBACK_RUNTIME_CONTRACT`, `createParameterFreeTool`.
- `plugin-sdk/provider-test-contracts` —
  `describeOpenAIProviderRuntimeContract`,
  `expectPassthroughReplayPolicy`, `runRealtimeSttLiveTest`,
  `expectExplicitVideoGenerationCapabilities`,
  `expectExplicitMusicGenerationCapabilities`,
  `mockSuccessfulDashscopeVideoTask`.
- `plugin-sdk/provider-http-test-mocks` — opt-in HTTP/auth Vitest mocks for
  provider plugins (`getProviderHttpMocks`,
  `installProviderHttpMockCleanup`).
- `plugin-sdk/test-env` — `withEnv`, `withFetchPreconnect`, `withServer`,
  `createTempHomeEnv`, `withTempHome`, `withTempDir`,
  `useFrozenTime`/`useRealTime`, `createMockServerResponse`.
- `plugin-sdk/test-fixtures` — `bundledPluginRoot`, `bundledPluginFile`,
  `createCliRuntimeCapture`, `typedCases`, `importFreshModule`,
  `writeSkill`, `makeAgentAssistantMessage`, `peekSystemEvents` /
  `resetSystemEventsForTest`, `sanitizeTerminalText`, `countLines`,
  `hasBalancedFences`.
- `plugin-sdk/test-node-mocks` — `mockNodeBuiltinModule` for narrow Node
  builtin Vitest mocks.

**Lint enforcement** for in-repo plugins (`pnpm check`):

1. No monolithic `openclaw/plugin-sdk` root barrel.
2. No direct `../../src/` imports from extensions.
3. No self-imports of the plugin's own `plugin-sdk/<name>` subpath.

**Contract test suites**: `pnpm test:contracts`,
`pnpm test:contracts:channels`, `pnpm test:contracts:plugins` iterate over
all discovered plugins and assert: plugin shape (id, name, capabilities),
setup wizard contract, session binding, outbound payload structure, inbound
handling, action handlers, threading, directory/roster, group policy,
status, registry, auth, auth-choice, catalog API, discovery, loader,
runtime, plugin shape (`docs/help/testing.md` lines 720-774).

## 9. Patterns AGH Should Adopt (verbatim or close)

- **Pattern: Scenario-as-markdown + coverage frontmatter**
  - Why it matters: machines and humans read the same artifact. Coverage
    drift is detectable mechanically (`qa coverage`). Scenario IDs are
    stable; doc/code references travel with the scenario.
  - AGH equivalent: a new `internal/qa/scenarios/<theme>/*.md` tree (or
    `.compozy/tasks/final-qa/scenarios/`), driven by a Go-side
    `qa-scenario-runner` that parses the same `qa-scenario` + `qa-flow` YAML
    fences. Map `coverage.primary` IDs to AGH surfaces under
    `internal/{daemon, autonomy, network, persist, transport, web, …}`.

- **Pattern: synthetic transport (qa-channel) for deterministic agent
  driving**
  - Why it matters: lets every scenario be expressed in the same vocabulary
    as a real channel, captures the transcript automatically, and drives the
    same SSE/UDS/HTTP surfaces production uses.
  - AGH equivalent: a `qa-channel` extension under `internal/transport/qa`
    that synthesizes ACP-shaped events, Slack-class targets
    (`dm:<peer>` / `channel:<room>` / `thread:<room>/<thread>`), and exposes
    the same SSE/UDS event stream the web UI subscribes to. Inbound
    injection through the daemon's HTTP/UDS surface; outbound capture via
    SSE bus; assert against `EventStore` rows.

- **Pattern: Operator identity + kickoff mission as repo-backed YAML**
  - Why it matters: every QA run starts in-character with the same mission
    framing; reports always end with worked/failed/blocked/follow-up, which
    reviewers can scan in seconds.
  - AGH equivalent: ship `internal/qa/operator.yaml` with the AGH operator
    persona/style and a kickoff body that maps the same four-bucket report
    contract. Reuse the Soul/Memory subsystem so the operator persona is a
    real Soul, not test config.

- **Pattern: Provider-mode tri-state (`mock-openai` / `aimock` / `live-frontier`)**
  - Why it matters: the same scenario file runs deterministically in CI,
    against record-replay protocol fixtures, and against real providers,
    without forking the test corpus.
  - AGH equivalent: gate scenario flow on `env.providerMode` against the
    AGH model registry. AGH already has a model registry under
    `internal/model`; expose `mock-acp` (deterministic ACP server),
    `acp-replay` (recorded session fixtures), and `live` modes.

- **Pattern: Tool-call evidence as pass criterion (criterion-2 parity gate)**
  - Why it matters: stops models from satisfying success criteria with
    plausible prose. Scenarios assert against the mock's
    `/debug/requests` log: `plannedToolName === 'read'` etc.
  - AGH equivalent: AGH's autonomy kernel records every tool call into the
    `tool_calls` ledger (per `internal/autonomy`). Scenarios should assert
    against ledger rows: e.g. `select count(*) from tool_calls where
    session_id = ? and name = 'fs.read' and ts >= scenario_start_ts`.

- **Pattern: qa-channel reconnect / dedupe + duplicate-window assertions**
  - Why it matters: catches double-delivery regressions across restart and
    reconnect cycles, which mocks alone never see.
  - AGH equivalent: scenarios assert that, after `daemon.restart` is forced
    and the SSE bus reconnects, the EventStore has exactly one row for the
    marker text and no duplicates inside a configurable
    `duplicateWindowMs`.

- **Pattern: Convex-style pooled live credential leasing**
  - Why it matters: lets release PR / nightly / parallel maintainer runs
    share one ChatGPT / Claude / Gemini login without colliding.
  - AGH equivalent: a `cred-broker` service or a thin SQLite-backed leasing
    table under `internal/qa/cred`; `acquire`/`heartbeat`/`release` APIs;
    pool kinds keyed by provider (`anthropic-api-key`, `claude-cli-oauth`,
    `gemini-cli`, etc.). Honor maintainer/CI role split.

- **Pattern: Frontier subset + bakeoff loop before regression**
  - Why it matters: cheap signal on whether a prompt/harness change regressed
    any provider family before the wider seed suite is even run.
  - AGH equivalent: `make qa-frontier` target that runs `approval-turn-…`,
    `model-switch-…`, `discovery-report` against a fixed
    GPT→Claude→Gemini bakeoff order; gate the wider suite on a clean
    frontier pass.

- **Pattern: per-lane Markdown report + JSON summary + observed-events
  artifact + combined log**
  - Why it matters: the four-artifact contract is the same shape every QA
    consumer expects; CI parses summary, humans read the markdown, bug
    reports attach observed events, debugging reads the log.
  - AGH equivalent: scenario runner writes `<lane>-qa-report.md`,
    `<lane>-qa-summary.json`, `<lane>-qa-observed-events.json` (events from
    the SSE bus), `<lane>-qa-output.log` under
    `.artifacts/qa/<run-id>/`. Redact bodies by default; opt-in capture
    flag for bug reports.

- **Pattern: Docs/Code refs in every scenario for traceability**
  - Why it matters: every behavior the QA suite protects has a single click
    to its spec and its implementation; scenario files become a living
    architecture map.
  - AGH equivalent: scenarios MUST list at least one `docsRefs` entry under
    `packages/site/content/docs/...` and at least one `codeRefs` entry
    under `internal/...` or `web/...`.

- **Pattern: Multipass / Docker disposable-VM lane**
  - Why it matters: a "works on Linux" guarantee without Docker leaking
    into the QA path; per-lane gateway workers.
  - AGH equivalent: a `make qa-multipass` target that boots a disposable
    Linux VM (or rootless container), builds AGH inside, runs the same
    `qa suite` against it, then mounts artifacts back to the host. Reuses
    the same scenario file format.

- **Pattern: forbidden-needle check for transcript leakage**
  - Why it matters: catches "tool failed", "internal error", "as an AI"
    style transport-error bleed-through (e.g.
    `qa/scenarios/character/character-vibes-c3po.md` lines 60-69).
  - AGH equivalent: every scenario can list `forbiddenNeedles[]`; the
    runner asserts these never appear in any outbound message (the full
    transport snapshot, not just the assertion target).

- **Pattern: scenario drives Control UI through `browser.request` seam**
  - Why it matters: replaces a separate Playwright lane with one runner.
    `webOpenPage`, `webWait`, `webSnapshot`, `webEvaluate` are the only
    helpers needed for transcript-persistence proofs.
  - AGH equivalent: AGH's web/ SPA already runs against the daemon HTTP
    API; scenarios should be able to drive a Chromium instance through a
    daemon-side helper instead of a separate test harness.

- **Pattern: `qa coverage` schema rules — lowercase dotted/dashed only,
  no scenario-shaped IDs, no source paths in schema**
  - Why it matters: makes coverage IDs reusable across scenarios and stops
    scenarios from minting one-off tags.
  - AGH equivalent: same regex; same Zod-style `superRefine` rejection of
    duplicates; same prohibition of the old flat `coverage: [id]` list
    shape.

- **Pattern: live-test isolated `HOME`**
  - Why it matters: live tests by default copy creds into a temp test home
    so unit fixtures cannot mutate real `~/.openclaw`. Set
    `OPENCLAW_LIVE_USE_REAL_HOME=1` only when you intentionally need it.
  - AGH equivalent: per the standing directive on worktree isolation, every
    QA run sets a unique `AGH_HOME`; live runs additionally stage provider
    auth into `PROVIDER_HOME` / `PROVIDER_CODEX_HOME` rather than reading
    `~/.codex` directly.

## 10. Patterns AGH Should NOT Adopt

- **TS / Vitest-shaped scenario runtime**. The scenario flow language is
  embedded JavaScript expressions evaluated against a TS scope (`expr:`,
  `lambda:`, `params:`). AGH is Go; the equivalent should be a
  predeclared, typed expression language (or simply Go calls registered as
  named ops) — not a JS sandbox.

- **`pnpm`-shaped command surface (`qa:lab:up`, `qa:lab:up:fast`,
  `qa:lab:watch`, `qa:otel:smoke`)**. AGH should expose one canonical
  `agh qa <subcommand>` plus `make` targets; do not import the four-way
  alias surface.

- **Docker-backed two-pane "QA Lab" web UI as the operator flow**. AGH's
  web UI is the operator surface; the scenario runner should drive it
  in-process via the same HTTP/SSE the user uses. A separate dashboard is
  unnecessary.

- **Bundled CLI auth mounting** (`OPENCLAW_DOCKER_AUTH_DIRS`,
  copying `.codex`, `.claude`, `.minimax` from host into container). This
  is provider-CLI-specific debt and conflicts with AGH's
  PROVIDER_HOME-isolation directive (SD on provider-home isolation).

- **Carbon / Sparkle / appcast / Parallels-guest plumbing**. openclaw QA
  has macOS/iOS/Android-app smoke layers (`docs/help/testing.md` lines
  221-256). AGH is a single Go binary; this is irrelevant.

- **JS-only `extensions/qa-lab/src/...` runtime helpers as scenario
  vocabulary**. The helper names (`waitForOutboundMessage`,
  `formatTransportTranscript`, `state.getSnapshot`) are fine concepts but
  the implementations are all TS/Node. AGH must rebuild them as Go.

- **Mintlify-specific docs-link rules** (the
  `docs/CLAUDE.md` Mintlify root-relative no-`.md` rule). AGH uses
  Fumadocs at `packages/site`; do not import openclaw's Mintlify
  conventions verbatim.

- **AIMock as a third provider mode**. openclaw is honest that it is
  additive and experimental ("does not replace the `mock-openai` scenario
  dispatcher"). AGH should standardize on two modes (deterministic mock +
  live) and skip the third lane until there is a forcing function.

- **`qa-channel` as a *bundled* plugin**. openclaw bundles qa-channel for
  packaging convenience. AGH should ship the synthetic transport as a
  build-tag-gated extension or test-only artifact so the production
  `agh` binary never advertises it (the npm tarball intentionally omits
  qa-lab; AGH should match).

- **Convex as the credential broker substrate**. Convex is an external
  managed service. AGH's broker should be local first (SQLite-backed) and
  optionally remote later. Keep the *contract* (acquire / heartbeat /
  release / admin add/remove/list, role-split secrets, lease TTL,
  heartbeat interval) but not the substrate.

## 11. Open Questions for AGH

1. **Canonical event names for QA assertions**. openclaw asserts against the
   mock's `/debug/requests` log and the synthetic transport's
   `state.getSnapshot().messages`. AGH's authoritative ledger is the
   EventStore + tool-call ledger under `internal/persist`; what is the
   stable subset of event names (e.g. `agent.message.outbound`,
   `tool.call.completed`, `cron.run.completed`) that scenarios are allowed
   to query, and what is the minimal index needed so a duplicate-window
   assertion is O(1)?

2. **Live credential leasing for AGH**. openclaw's broker uses Convex with a
   maintainer/CI role split, kind partitioning, and a heartbeat protocol.
   AGH needs an equivalent that respects the PROVIDER_HOME isolation rule
   and that does not require an external SaaS. SQLite-backed local broker?
   Per-worktree env-var injection?

3. **Bakeoff order for AGH-supported providers**. openclaw runs
   GPT → Claude → Gemini in that order. AGH's primary support matrix
   includes Claude Code, OpenClaw, Hermes; what is the canonical bakeoff
   order, and which AGH-equivalents of `approval-turn-tool-followthrough`,
   `model-switch-tool-continuity`, and `source-docs-discovery-report`
   should gate harness changes?

4. **Scenario flow runtime in Go**. openclaw evaluates inline JS with
   `expr:` and `lambda:`. AGH must pick between (a) a Starlark / CEL-style
   expression language, (b) a fixed registry of named Go ops with typed
   args, (c) embedding YAML scenarios into Go test files via codegen. The
   answer drives whether scenarios live in `.md` or stay co-resident with
   Go test code.

5. **Web QA inside the same scenario file vs. a separate Playwright lane**.
   openclaw drives Control UI through gateway-side `browser.request`
   helpers (`webOpenPage`, `webSnapshot`, `webEvaluate`) within the same
   `qa-flow`. AGH's web/ SPA is React/Vite; should the scenario runner
   spawn Playwright/Chromium itself, or should the daemon expose an
   equivalent `browser.*` RPC that scenarios call?

6. **Coverage ID taxonomy**. openclaw uses behavior-shaped IDs
   (`memory.recall`, `runtime.compaction`, `scheduling.cron`,
   `channels.dm`). AGH has a different surface vocabulary (autonomy
   kernel, ACP, AGH Network, soul, capabilities, hooks, registries,
   bundles). What is the AGH coverage-ID list, and is it tied to package
   paths, behavior names, or both?

## Citations

- `.resources/openclaw/qa/README.md` — top-level index of QA scenarios:
  `scenarios/index.md`, themed `scenarios/<theme>/*.md`,
  `frontier-harness-plan.md`, `convex-credential-broker/`. Defines
  `qa suite` vs `qa manual` vs `qa coverage` workflow split.
- `.resources/openclaw/qa/scenarios.md` — pointer file confirming canonical
  scenario source lives in `scenarios/index.md` and themed subdirs.
- `.resources/openclaw/qa/scenarios/index.md` — pack-level
  `qa-pack` block: operator identity (Dev C-3PO persona/style/markdown),
  kickoff task, theme directory definitions, coverage-ID rules.
- `.resources/openclaw/qa/new-scenarios-2026-04.md` — round-2 scenario
  expansion list (10 candidates, top-3 promotion shortlist). Useful as a
  template for AGH "follow-up scenarios".
- `.resources/openclaw/qa/frontier-harness-plan.md` — frontier subset (3
  scenarios), bakeoff order GPT→Claude→Gemini, tuning loop, scoring
  dimensions, manual personality lane.
- `.resources/openclaw/qa/convex-credential-broker/README.md` — six HTTP
  endpoints, pool-by-kind, least-recently-leased selection, retention
  policies, payload validation rules per kind. Defines the contract AGH
  should mirror.
- `.resources/openclaw/qa/scenarios/agents/instruction-followthrough-repo-contract.md`
  — repo-contract followthrough; seeded `AGENT.md` / `SOUL.md` files;
  `expectedReplyAll` + `expectedArtifactAll/Any` assertion shape;
  `forbiddenNeedles` permission-bouncing detector; pre-tool plannedToolName
  ordering check (3 reads before any write).
- `.resources/openclaw/qa/scenarios/agents/subagent-handoff.md` — parity
  criterion-2 evidence requirement: `sessions_spawn` must be observed in
  the mock debug log, scoped by scenario-unique prompt substring to avoid
  false positives.
- `.resources/openclaw/qa/scenarios/agents/subagent-fanout-synthesis.md` —
  retry-attempt loop, child-session store inspection, two
  `sessions_spawn` calls with distinct labels as criterion-2 evidence.
- `.resources/openclaw/qa/scenarios/channels/dm-chat-baseline.md` — minimal
  baseline: synthetic inbound DM message via `state.addInboundMessage`,
  outbound assertion by conversation id.
- `.resources/openclaw/qa/scenarios/channels/qa-channel-reconnect-dedupe.md`
  — readiness-cycle dedupe assertion: exactly one outbound before the
  reconnect, exactly one after; no replay of the first reply.
- `.resources/openclaw/qa/scenarios/channels/thread-follow-up.md` —
  threading proof: thread-create action, threadId scoped outbound,
  assertion that no thread reply leaks into the root channel.
- `.resources/openclaw/qa/scenarios/character/character-vibes-c3po.md` —
  multi-turn natural transcript capture, `forbiddenNeedles` for transport
  errors and "as an ai" signals; later judge model grades naturalness.
- `.resources/openclaw/qa/scenarios/character/character-vibes-gollum.md` —
  same shape as C-3PO; SOUL.md persona authored as workspace artifact;
  judge-graded naturalness/vibe/humor.
- `.resources/openclaw/qa/scenarios/config/config-apply-restart-wakeup.md`
  — `applyConfig` with `deliveryContext`, restart-sentinel wake message
  through qa-channel, gateway+channel readiness gates after restart.
- `.resources/openclaw/qa/scenarios/config/config-patch-hot-apply.md` —
  `patchConfig` toggles `skills.entries.<name>.enabled`; pre/post
  `readSkillStatus` snapshots used as drift evidence.
- `.resources/openclaw/qa/scenarios/media/image-understanding-attachment.md`
  — image attachment as `mimeType: image/png` + `content: <base64>`;
  `imageInputCount` field in mock debug log is the equivalent of a
  tool-call assertion for vision scenarios.
- `.resources/openclaw/qa/scenarios/media/native-image-generation.md` —
  `image_generate` must appear in `tools.effective` after config patch;
  `resolveGeneratedImagePath` proves a real file was written.
- `.resources/openclaw/qa/scenarios/memory/memory-recall.md` — durable
  memory canary across context switch; explicit comment block defending
  the prose-only assertion shape because real models can recall via
  conversation context OR via a memory tool, both legitimate.
- `.resources/openclaw/qa/scenarios/memory/memory-tools-channel-context.md`
  — channel-shaped recall; `forceMemoryIndex` helper; `memory_search` +
  `memory_get` tool-plan assertions in mock mode.
- `.resources/openclaw/qa/scenarios/memory/memory-dreaming-sweep.md` —
  longest scenario; enables dreaming via `patchConfig`, seeds
  `MEMORY.md` and synthetic transcript JSONL, forces dreaming cron, asserts
  `# Light Sleep` / `# REM Sleep` artifacts and durable-memory promotion.
- `.resources/openclaw/qa/scenarios/models/model-switch-tool-continuity.md`
  — alternate-model handoff, post-switch read tool-plan check, model field
  asserted equal to `alternate?.model`.
- `.resources/openclaw/qa/scenarios/models/anthropic-opus-api-key-smoke.md`
  — `requiredProvider` + `requiredModel` gating; `ANTHROPIC_API_KEY`
  presence assertion; live-frontier-only.
- `.resources/openclaw/qa/scenarios/plugins/skill-visibility-invocation.md`
  — workspace skill written via `writeWorkspaceSkill`, visibility checked
  via `findSkill(skills, name)?.eligible`, marker assertion proves the
  skill instructions reached the agent.
- `.resources/openclaw/qa/scenarios/plugins/skill-install-hot-availability.md`
  — pre-install absence check, post-install hot eligibility check without
  restart, marker reply.
- `.resources/openclaw/qa/scenarios/plugins/plugin-lifecycle-hot-reload.md`
  — disable+re-enable via two `patchConfig` calls; `waitForQaChannelReady`
  between toggles to avoid stale state.
- `.resources/openclaw/qa/scenarios/plugins/mcp-plugin-tools-call.md` —
  real MCP-side memory_search via `callPluginToolsMcp` helper; asserts the
  returned `result.content[]` includes the expected needle.
- `.resources/openclaw/qa/scenarios/runtime/approval-turn-tool-followthrough.md`
  — frontier subset member; pre-action prompt + short approval; tool call
  must follow the approval, not before; expectedReplyAny needle list.
- `.resources/openclaw/qa/scenarios/runtime/compaction-retry-mutating-tool.md`
  — large seeded context (160 lines mock / 2200 lines live), mutating
  write, replay-unsafety phrase asserted in reply, `compactionCount` from
  raw session store reported in detailsExpr.
- `.resources/openclaw/qa/scenarios/runtime/empty-response-recovery-replay-safe-read.md`
  — mock-only; injects empty response; asserts the runtime injects "did
  not produce a user-visible answer" continuation prompt.
- `.resources/openclaw/qa/scenarios/runtime/gateway-restart-inflight-run.md`
  — start a run, force restart, assert ≤1 interrupted-marker delivery and
  exactly one recovery-marker delivery on the next turn.
- `.resources/openclaw/qa/scenarios/runtime/streaming-final-integrity.md` —
  exactly one final marker outbound; no token-delta-shaped partials in
  the channel; uses regex on outbound text to detect partial leakage.
- `.resources/openclaw/qa/scenarios/runtime/runtime-inventory-drift-check.md`
  — `tools.effective` and `skills.status` aligned with runtime; deny
  patch removes `image_generate` from effective tools.
- `.resources/openclaw/qa/scenarios/runtime/otel-trace-smoke.md` —
  `gatewayConfigPatch.diagnostics.otel` enables traces; `plugins:
  [diagnostics-otel]` block declares plugin requirements; release-critical
  span set documented in qa-e2e-automation.md.
- `.resources/openclaw/qa/scenarios/scheduling/cron-one-minute-ping.md` —
  `cron.add` then `cron.run mode: force`, channel announcement delivery,
  marker round-trip.
- `.resources/openclaw/qa/scenarios/scheduling/cron-single-run-no-duplicate.md`
  — duplicate-window dedupe assertion; `cron.runs` paginated history
  inspected for exactly one completed entry.
- `.resources/openclaw/qa/scenarios/security/secret-redaction-tool-logs.md`
  — fake-secret fixture written, agent prompted to not echo, every new
  outbound asserted free of the fake secret string.
- `.resources/openclaw/qa/scenarios/ui/control-ui-qa-channel-image-roundtrip.md`
  — embedded Control UI driven via `webOpenPage` / `webWait` /
  `webSnapshot` / `webEvaluate`; transcript persistence on fresh page
  load; image roundtrip with `requiredColorGroups` matcher.
- `.resources/openclaw/qa/scenarios/workspace/lobster-invaders-build.md` —
  small build artifact at `./lobster-invaders.html`; pre-write read
  evidence asserted in mock log.
- `.resources/openclaw/qa/scenarios/workspace/source-docs-discovery-report.md`
  — frontier subset member; agent must read repo files before writing the
  worked/failed/blocked/follow-up report; criterion-2 read tool-call
  assertion against scenario-unique prompt substring.
- `.resources/openclaw/docs/concepts/qa-e2e-automation.md` — full QA stack
  reference: command surface table, operator flow (`qa:lab:up`), live
  transport coverage matrix (Matrix/Telegram/Discord), Convex credential
  pool semantics, repo-backed seed conventions, provider mock lanes,
  transport adapter contract, scenario helper names.
- `.resources/openclaw/docs/concepts/qa-matrix.md` — Matrix QA reference:
  Tuwunel homeserver provisioning, profile catalog (`fast` /
  `transport` / `media` / `e2ee-smoke` / `e2ee-deep` / `e2ee-cli`),
  scenario IDs, env vars (`OPENCLAW_QA_MATRIX_*`), output artifact
  layout, triage tips.
- `.resources/openclaw/docs/help/testing.md` — full testing kit: unit /
  integration / e2e / live suites; Docker runners; `pnpm openclaw qa
  suite` operator behavior; `qa-channel` runners summary; QA-Lab CI
  workflows; Convex credential pool env vars (`OPENCLAW_QA_CONVEX_*`).
- `.resources/openclaw/docs/help/testing-live.md` — live (network-touching)
  test reference: model matrix / CLI backends / ACP / media providers.
  Defines isolated-`HOME` policy and `OPENCLAW_LIVE_USE_REAL_HOME`
  override.
- `.resources/openclaw/docs/channels/qa-channel.md` — synthetic transport
  config surface (`baseUrl`, `botUserId`, `botDisplayName`, `allowFrom`,
  `pollTimeoutMs`, `defaultTo`, `actions.*`, multi-account `accounts`).
  `pnpm qa:e2e` self-check entry point.
- `.resources/openclaw/docs/reference/test.md` — Vitest CLI variants
  (`test:force`, `test:coverage`, `test:changed`, `test:e2e`, `test:live`,
  `test:docker:all`, `test:contracts:*`, `test:perf:*`).
- `.resources/openclaw/docs/plugins/sdk-testing.md` — focused-subpath plugin
  test SDK; lint enforcement (no monolithic root barrel, no `src/`
  imports); contract test surfaces; testing patterns (per-instance stubs,
  registration-contract loader smoke tests, runtime mock injection).
- `.resources/openclaw/test/helpers/AGENTS.md` — shared test helper
  boundary: helpers go through `src/test-utils/bundled-plugin-public-surface.ts`,
  no repo-relative imports into `extensions/**`, prefer
  `loadBundledPluginApiSync(...)` family for eager plugin surface access.
- `.resources/openclaw/extensions/qa-lab/src/scenario-catalog.ts` (lines
  23-232) — Zod schemas for `qa-scenario` and `qa-flow` blocks.
  Authoritative source for every metadata field, action kind, and
  validation rule.
- `.resources/openclaw/extensions/qa-lab/src/coverage-report.ts` — pure
  function `buildQaCoverageInventory(scenarios)` deriving themes,
  surfaces, primary/secondary split, overlapping/missing coverage from
  scenario frontmatter.
- `.resources/openclaw/extensions/qa-lab/src/agentic-parity-report.ts`
  (lines 343-430) — parity-report scoping by required scenario titles,
  fail criteria for missing or failing required parity scenarios,
  fake-success detector via `SUSPICIOUS_PASS_FAILURE_TONE_PATTERNS`.
- `.resources/openclaw/CLAUDE.md` (alias for `AGENTS.md`) — repo-wide
  rules: extension boundary, QA expectations, "QA-Lab - All Lanes"
  CI workflow naming, parity gate, shared `qa-live-shared` GitHub
  environment for Convex CI credential leases.
