# J-cross-workspace-access — Reach another workspace under the session's permission mode

An agent working in one workspace needs something that lives in another: a workspace read, a task to
claim, a coordination setting. The session's own permission mode is the only
authority. `approve-all` crosses without asking, `deny-all` never crosses and never asks, and
`approve-reads` asks the operator once at the native-tool seam and denies with a hint everywhere
else. In a healthy event store, the operator can read the decision afterwards; an audit append
failure is warned and never changes the access decision.

**Delegation is no longer one of these seams.** The `compozy spawn` verb and its HTTP, UDS and
native-tool routes were deleted with the agent-comms change; the surface that replaced them —
`compozy call` — does not participate in the permission-mode funnel at all. A cross-workspace call
or message target is a **hard denial** (`call_workspace_denied`) before any side effect, in every
mode including `approve-all`, and it never raises a prompt. The contrast is the point of this
journey now: the consent seams still negotiate, and the delegation seam simply refuses.

```mermaid
flowchart TD
    E1[Entry: native tool call naming another workspace] --> CAN[Target is canonicalized to a registry workspace id]
    E2[Entry: agent-driven CLI verb with --workspace on another workspace] --> CAN
    E3[Entry: agent HTTP or UDS identity route scoped to another workspace] --> CAN
    E4[Entry: claim the next task run in another workspace] --> CAN
    E5[Entry: call or message an agent in another workspace] --> HARD[call_workspace_denied — a hard denial before any side effect, in EVERY mode, with no prompt: delegation is not a consent seam]
    E6[Entry: read or set network coordination in another workspace] --> CAN
    E7[Entry: the deleted spawn surfaces — compozy spawn --workspace, POST /api/agent/spawn, compozy__session_spawn] --> GONE[Absent: a normal unknown-command or not-found, never a compatibility alias into this funnel]
    CAN --> POL{Which authority decides?}
    POL -->|operator, or the target is home| ALLOW
    POL -->|mode approve-all| ALLOW[Crossing proceeds, no prompt]
    POL -->|mode deny-all| DENY[Denied at every seam, no prompt anywhere]
    POL -->|mode approve-reads, session answer already cached| CACHED{Cached session answer}
    CACHED -->|allow| ALLOW
    CACHED -->|reject| DENY
    POL -->|mode approve-reads, no answer, seam other than the live native tool| DENY
    POL -->|mode approve-reads, no answer, live native tool| ASK[One pending permission offering allow_once, allow_session, reject_once, reject_session]
    ASK -->|allow_once| ONCE[This call proceeds — the next crossing asks again]
    ASK -->|reject_once| RONCE[This call is denied — the next crossing still asks]
    ASK -->|allow_session| SESSALLOW[Answer held in daemon memory only]
    ASK -->|reject_session| SESSREJ[Answer held in daemon memory only]
    ASK -.->|nobody answers before the approval timeout| TO[Abandon: the call denies and no answer is stored]
    TO --> DENY
    ONCE --> ALLOW
    RONCE --> DENY
    SESSREJ --> DENY
    SESSALLOW --> REUSE[Later task-claim and coordination crossings succeed with no further prompt — a call target in another workspace is still hard-denied]
    REUSE --> HARD
    ALLOW --> AUD[Side effect: best-effort workspace.access_granted event names target, seam, source, and mode]
    DENY --> HINT[Denial carries the permission-mode hint — native denials report workspace_access_denied and agent CLI verbs exit 77]
    HINT --> AUD2[Side effect: best-effort workspace.access_denied event names target, seam, source, and mode]
    HINT -.->|agent stops retrying and reports the block| AB[Abandon: operator raises permissions.mode or answers the prompt, then the agent retries]
    AB -.-> CAN
    REUSE --> STOP[Operator stops the session, or the daemon restarts]
    STOP --> EXP[The cached answer is gone — the next crossing asks again]
    AUD --> END[True end: the foreign workspace holds the result, persisted audit records explain their decisions, no consent outlived its session, and the delegation seam refused every time without ever asking]
    AUD2 --> END
    EXP --> END
    HARD --> END
    GONE --> END
```

```yaml
journey:
  id: J-cross-workspace-access
  name: "Reach another workspace under the session's permission mode"
  value_statement: "An agent can work across workspace boundaries when its operator allows it, is blocked deterministically when they don't, and a healthy event store gives the operator an audit trail of the decisions."
  personas: [Ada, Bruno]
  entry_points:
    - url: "compozy__workspace_info / compozy__memory_* / compozy__task_run_claim_next with a foreign workspace input"
      origin: direct
    - url: "compozy task next --workspace / compozy network coordination status --workspace"
      origin: direct
    - url: "POST /api/agent/tasks/claim-next, GET /api/agent/me over HTTP and UDS"
      origin: direct
    - url: "the delegation seam, which refuses instead of negotiating: compozy call ses_foreign 'cross-scope attempt'; compozy message send ses_foreign 'cross-scope attempt'; HTTP/UDS POST /api/workspaces/{workspace_id}/calls with target.session_id=ses_foreign; HTTP/UDS POST /api/workspaces/{workspace_id}/messages with to.session_id=ses_foreign; compozy__agent_call and compozy__agent_message with the same foreign target"
      origin: direct
    - url: "the deleted spawn surfaces, probed as negatives: compozy spawn --workspace; POST /api/agent/spawn; the UDS spawn route; compozy__session_spawn"
      origin: direct
    - url: "GET|PUT /api/workspaces/:workspace_id/network-coordination"
      origin: direct
    - url: "compozy session approve <session-id> --request-id <request-id> --decision <decision> / POST /api/workspaces/:workspace_id/sessions/:session_id/approve"
      origin: in-app-nav
    - url: "compozy logs --type workspace.access_granted|workspace.access_denied, GET /api/logs, compozy__logs, compozy__observe_search"
      origin: direct
  actions:
    - step: 1
      verb: "Name another workspace from a session whose agent is approve-all"
      expected_observable: "The crossing succeeds at the native-tool, task-claim, and coordination seams with no prompt, and the work lands in the named workspace; a call or message aimed at that same foreign workspace is still hard-denied with call_workspace_denied, because approve-all is not authority over delegation"
    - step: 2
      verb: "Repeat the same crossings from a deny-all session"
      expected_observable: "Every seam denies with the permission-mode hint, nothing prompts anywhere, and native denials report reason workspace_access_denied while agent CLI verbs exit 77"
    - step: 3
      verb: "Repeat from an approve-reads session and answer the native-tool prompt"
      expected_observable: "Exactly one pending permission offers allow once, allow for this session, reject once, and reject for this session; the non-tool seams deny with the same hint and never prompt"
    - step: 4
      verb: "Answer allow for this session, then cross at a seam that never prompts"
      expected_observable: "Task claim and coordination crossings now succeed without asking, and no approval appears in any list or revoke surface; the call and message seams still refuse, so a session answer can never be laundered into delegation authority"
    - step: 5
      verb: "Stop the session, or restart the daemon, and cross again"
      expected_observable: "The first crossing prompts again — the session answer did not survive"
    - step: 6
      verb: "Read the audit trail as the operator"
      expected_observable: "In a healthy event store, each policy evaluation appears once, scoped to the actor's own workspace, naming the target workspace, seam, decision source, and mode; an append failure warns without changing access"
    - step: 7
      verb: "Probe the deleted spawn surfaces from every mode"
      expected_observable: "The CLI verb, HTTP route, UDS route and native tool are genuinely absent — a normal unknown-command or not-found — with no compatibility alias that would route an old spawn crossing back into this consent funnel"
  goal:
    observable: "Each crossing attempt resolves to exactly the outcome the session's mode promises, delegation refuses in every mode, and persisted audit records let the operator name why"
    side_effects: [workspace-access-audit-event, foreign-workspace-read-or-mutation, session-consent-cached-in-memory]
  true_end_state: "Allowed work is observable in the target workspace and gives the actor the same rights it has at home; denied work changed nothing anywhere; every successfully appended audit record names its decision; stopping the session leaves no consent behind — the next crossing starts from the mode again; and no permission mode or cached session answer ever produced a cross-workspace call, message, or spawn."
  exit:
    natural: "The agent continues its task in the workspace it was allowed to reach, or reports the block and waits for the operator."
  abandonment:
    - at_step: 3
      how: "Nobody answers the native-tool prompt before the approval timeout expires."
      resume: "The call denies and stores no answer; a later crossing raises a fresh prompt rather than inheriting a stale one."
    - at_step: 2
      how: "The agent hits a final denial, stops retrying, and surfaces the block to the operator."
      resume: "The operator raises the agent's permissions.mode or answers the prompt; the agent retries the same crossing and it now resolves under the new authority."
  crosses: [native-tools, CLI, HTTP, UDS, session-identity, task-claim, calls-workspace-denial, network-coordination, approval-bridge, event-store, site-docs, official-skill]
```

Taxonomy note: journeys, functional checks, and edge/error states are all in scope — the denial hint,
reason code, exit code, and audit payload are the observables. Experiential and responsiveness lenses
apply only to the operator's prompt-answering and audit-reading surfaces, not to the agent's
structured seams. Cross-cutting regression rides the adjacent canary journey
`J-operate-workspace-context`, which owns same-workspace resolution and pre-handler binding.
