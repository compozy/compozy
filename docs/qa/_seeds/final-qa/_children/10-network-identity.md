---
name: 10-network-identity
title: AGH Network Participation + Identity — Real-Provider QA Seed
description: Behavior-first QA seed for Local/Live participation, commit-first in-process delivery, bounded wakes, workspace isolation, identity authorization, and HTTP/UDS/CLI/Web parity.
type: final-qa-child
module: network-identity
provider_lanes: [claude-code]
authoritative_runtime_truth: internal/CLAUDE.md
references:
  - ../../../../rfcs/003_agh-network-v0.md
  - ../../../../rfcs/004_agh-network-v1.md
  - ../../../scenarios/NB-execution-participation-defaults.md
  - ../../../scenarios/NB-run-bounded-live-collaboration.md
  - ../../../scenarios/NB-network-live-config-lifecycle.md
  - ../../../scenarios/NB-coordination-invitation-future-runs.md
---

# 10 — AGH Network Participation + Identity

## 1. Module scope

This module owns the seams between explicit execution participation, durable Network conversations,
bounded model activation, daemon-authoritative caller identity, and public management surfaces.

The current release has no remote carrier. All accepted messages are committed to SQLite before
eligible local recipients are notified in-process. A QA plan that requires two daemons to exchange
Network messages is invalid for this release.

In scope:

- `internal/network/participation`: typed request/spec validation and resolution
- `internal/network`: envelope validation, durable acceptance, recipient dispositions, wake admission,
  coalescing, settlement, usage, and conversation reads
- `internal/agentidentity`: daemon-authoritative session/agent/workspace authorization
- `internal/task`: `network_wake` runs on the single task-run claim substrate
- `internal/api`, `internal/cli`, `internal/tools`: structured management and gated coordination surfaces
- `web/`: participation controls, coordination invitation, conversation/usage, empty states, and settings

Out of scope:

- cross-installation delivery, federation, offline remote addressing, or a remote carrier
- cryptographic v1 proof enforcement; RFC 004 remains a future profile
- spend caps or a public mailbox mode
- replacing task ownership with conversation state

## 2. Authoritative invariants

| Coverage ID | Invariant |
| --- | --- |
| `network.local-default` | Omitted participation resolves one immutable `Local` / `built_in_local` snapshot. |
| `network.local-zero-state` | Local creates no channel lease, Network prompt/environment/toolset, wake, or Network usage. |
| `network.live-explicit` | Live requires an explicit typed request, an authorized channel strategy, availability, and bounds within administrative ceilings. |
| `network.availability-not-enrollment` | `network.enabled` only controls availability; enabling it never changes an execution snapshot. |
| `network.children-independent` | Spawned, review, and detached children resolve participation independently and never inherit a conversation. |
| `network.commit-first` | Conversation plus recipient dispositions commit atomically before any in-process notification. |
| `network.wake-eligibility` | Only an addressed or mentioned `say` may wake a Live execution. Other kinds may persist without activation. |
| `network.wake-bounded` | One owner has at most one in-flight wake; coalescing, depth, count, token, and wall-time ceilings are finite and visible. |
| `network.wake-recoverable` | An admitted unclaimed wake survives daemon restart and resumes through the task-run claim substrate. |
| `network.usage-truthful` | Usage is provider-reported actual usage or `usage_unavailable`; no estimate is presented as fact. |
| `network.task-authority` | Conversation is evidence. Task/run state remains the sole executable-work authority. |
| `network.workspace-isolation` | Channels, conversations, usage, invitations, and wake ledgers are workspace-scoped through store, API, SSE, cache, CLI, and Web. |
| `network.coordination-future-runs` | Coordination changes affect future coordinated runs only and never mutate an active run. |
| `network.status-truthful` | Status is `disabled`, `ready`, or `active` and exposes no removed listener, remote-peer, or queue-worker state. |
| `identity.daemon-authoritative` | Session, agent, and workspace identity are validated against daemon state; spoofed or stale values fail deterministically. |
| `identity.secret-redaction` | Raw claim tokens and provider secrets never appear in Network bodies, metadata, usage, logs, events, or API errors. |
| `network.surface-parity` | HTTP and UDS share handlers; CLI and Web project the same records and error codes. |

## 3. QA lab contract

Start every pass with `agh-qa-bootstrap`. Use one fresh manifest and its generated `AGH_HOME`,
workspace, HTTP/UDS endpoints, Web proxy target, provider home, evidence path, and teardown command.

Rules:

1. Use one daemon for Network delivery. A second isolated daemon may be used only to prove workspace
   or process isolation, never as a message recipient.
2. Preserve the provider contract from the manifest. Native CLI providers with operator-home policy
   keep the operator login; isolated/bound credentials use the manifest provider home.
3. Register every daemon, Web server, browser, watcher, or long-lived helper under
   `<QA_OUTPUT_PATH>/qa/pids/`.
4. Run config writes sequentially against one lab home.
5. End every path with the manifest teardown command and retain `teardown.json` with `clean: true`.

## 4. Scenario catalog

The scenario IDs remain `NET-01..21` so the master and cross-cutting QA seeds keep stable references.
The living `docs/qa/scenarios/` files remain the authoritative behavior inventory.

| ID | Scenario | Primary proof |
| --- | --- | --- |
| NET-01 | Plain session/task/Loop/automation owners default Local | Snapshot is Local; channel, wake, prompt, tools, and usage deltas remain zero. |
| NET-02 | Explicit authorized Live execution starts | Immutable Live snapshot and bounds appear consistently over HTTP, UDS, CLI, and run detail. |
| NET-03 | Live rejected while unavailable | Typed 4xx occurs before first prompt; no partial Network state persists. |
| NET-04 | Unknown/unauthorized channel strategy rejected | Stable error code; no owner record, lease, or wake is created. |
| NET-05 | Secret and claim-token redaction | Forbidden-needle scan across body, event, log, usage, support, and error evidence is empty. |
| NET-06 | Thread ordering and duplicate acceptance | Committed order is stable; replaying the same message ID creates no second conversation row or wake. |
| NET-07 | Direct addressed `say` wakes once | Message commits, one recipient disposition admits, one bounded wake prompts the target. |
| NET-08 | Discovery/control messages do not wake | `greet`, `whois`, `capability`, `receipt`, and `trace` may persist with zero model activation. |
| NET-09 | Unsupported protocol version rejected | Invalid protocol is a typed rejection with no conversation or wake side effect. |
| NET-10 | Runtime has no remote-delivery lifecycle | Status and process inspection expose no remote listener, connection, or worker state. |
| NET-11 | Dependency hard-cut audit | Production imports and module graph contain no retired remote-delivery dependency or subject grammar. |
| NET-12 | Daemon-authoritative identity succeeds | Active session/agent/workspace identity authorizes the intended structured surface only. |
| NET-13 | Stale, mismatched, and spoofed identity fail | Each branch returns its documented code and non-zero CLI exit without data disclosure. |
| NET-14 | Workspace data isolation | Cross-workspace list/read/usage/SSE/cache probes return no foreign datum. |
| NET-15 | Restart preserves committed conversation and admitted wake | Conversation survives; unclaimed admitted work recovers exactly once. |
| NET-16 | Causal burst coalesces | Ten same-root messages produce one prompt and visible coalescing counters. |
| NET-17 | Depth and total budgets exhaust visibly | Further messages persist but do not prompt; reason and consumed bounds are inspectable. |
| NET-18 | Cancel/deadline settlement is terminal | One outcome wins CAS; usage is actual or unavailable; no stuck in-flight owner remains. |
| NET-19 | Real-provider Live collaboration | A real provider receives untrusted Network context once and returns output tied to the admitted wake. |
| NET-20 | Coordination invitation changes future runs only | Accept/dismiss persist; active run snapshot stays unchanged; next coordinated run resolves per setting. |
| NET-21 | Web settings/status/conversation parity | UI states, controls, empty explanations, history, and usage match structured daemon truth. |

## 5. Required execution detail

### NET-01 / NET-02 control pair

Capture a before/after Network status and usage snapshot. Create the same deterministic task twice:
once without participation and once with an explicit authorized Live request. Compare persisted owner
snapshots, provider prompt/environment/tool descriptors, channel catalog, wake ledger, and usage.

The Local run is not allowed a merely smaller Network projection. It must have none.

### NET-06 / NET-07 / NET-08 delivery matrix

Use fixed envelope IDs and timestamps. Send:

- a public-thread `say` without an address or mention
- an addressed direct-room `say`
- a mentioned thread `say`
- one message of every non-`say` kind
- an exact replay of the addressed `say`

Assert status code and body for every public call. Read the committed conversation and recipient
dispositions before inspecting wake effects. One eligible message may create one wake; the replay and
control kinds may not.

### NET-14 workspace isolation matrix

Create workspaces A and B with the same channel slug. Generate one conversation, one usage row, one
coordination setting, and one invitation state in each. Probe list/read/status/usage over HTTP, UDS,
CLI, Web query cache, and SSE using the opposite workspace identity. Any foreign identifier is a
critical failure.

### NET-15 / NET-18 recovery and settlement

Pause the wake runner after durable admission but before claim. Stop the daemon through its public
surface, restart from the same lab home, and prove exactly one claim. In a separate run, cancel or hit
the wall-time deadline during provider work. Assert one terminal settlement, no raw claim token, no
owner left in-flight, and truthful usage state.

### NET-19 real-provider lane

Use the manifest-approved real provider. Put a unique nonce in an addressed Network `say`; require the
agent to acknowledge that nonce and perform a bounded, read-only workspace action. Evidence must join:

- accepted message and recipient disposition
- wake task-run claim/settlement
- provider transcript/event lineage
- actual usage or `usage_unavailable`
- run-detail conversation projection

Do not inject the nonce through the initial user prompt. That would prove prompt plumbing, not Network
activation.

### NET-20 / NET-21 user-visible lane

Use the browser against the isolated Web proxy target. Capture the invitation before and after accept,
reload after dismiss, and inspect an already-active run to prove it did not change. Start a future
coordinated run and compare its snapshot with the setting. Capture Network settings in disabled,
ready, and active states plus empty and populated run-conversation states.

## 6. Evidence bundle

Each executed scenario writes under the manifest QA output path:

- `requests/`: exact HTTP/UDS/CLI request and response bodies
- `state/`: status, owner snapshot, conversation, disposition, wake, and usage reads
- `events/`: bounded event/SSE windows with correlation IDs
- `provider/`: redacted real-provider transcript and usage result where applicable
- `browser/`: screenshots and interaction log for NET-20/21
- `forbidden-needles.txt`: zero-hit secret/claim-token scan
- `teardown.json`: mandatory `clean: true`

Every verdict cites exact evidence paths. A screenshot alone cannot prove persistence, isolation,
settlement, or structured-surface parity.

## 7. Stop conditions

Stop the affected lane and file a content-addressed bug when any of these occurs:

- Local creates any Network state or model affordance
- availability silently enrolls an execution
- an accepted message is missing after restart
- a duplicate or coalesced message creates a second prompt
- a non-addressed/non-mentioned message wakes the model
- bounds are exceeded without an explicit exhaustion outcome
- usage is estimated but labeled actual
- workspace B can observe workspace A data
- an active run changes after coordination setting mutation
- any raw claim token or provider secret appears in evidence
- teardown is not clean

## 8. Cross-surface impact checklist

- Native tools: Live-only coordination descriptors and `not_participating` diagnostics match the live
  catalog; Local receives none.
- Extensibility and hooks: extension Live requirements require explicit activation confirmation;
  declared channels never enroll. Participation hooks may deny or narrow but cannot silently widen.
- Workspace data isolation: owner snapshots, conversations, invitations, usage, wakes, SSE, and caches
  remain workspace-scoped.
- Official AGH skill: `skills/agh/references/network.md`, task orchestration, runtime operations, and
  native-tool guidance match the observed public contract.
