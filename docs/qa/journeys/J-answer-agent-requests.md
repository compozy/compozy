# J-answer-agent-requests — Answer an agent's requests once

An operator supervising a live session answers a native-tool approval prompt or an agent question
exactly once and is never asked the same thing again: `allow_always`/`reject_always` persist a
durable grant that survives daemon restart and is revocable, and `agh__clarify` blocks the agent
until the operator answers (or the timeout resolves the explicit unanswered sentinel). Covers
US-001 (D1, ADR-001) and US-002 (D7, ADR-001).

```mermaid
flowchart TD
    E1[Entry: Web session timeline] --> P[Agent calls a native tool under the three-mode ceiling]
    E2[Entry: CLI pending approvals/clarifications] --> P
    P --> M{Mode allows a prompt?}
    M -->|deny-all| DENY[Call denied — stored allow grant never overrides the ceiling]
    M -->|prompt| A[Approval prompt shown]
    A -->|allow_always| G[Side effect: durable grant row at the most-specific key + canonical event]
    A -->|reject_always| RJ[Side effect: durable reject grant]
    A -->|allow_once| ONCE[Nothing persists]
    G --> F[Identical follow-up call auto-approves with zero prompt]
    RJ --> FD[Follow-up auto-denies with a deterministic error]
    F --> R[Daemon restart]
    R --> F2[Matching call still auto-approves — grant is durable]
    F2 --> RV[Operator revokes via Web/CLI/native set surfaces]
    RV --> RP[Next matching call prompts again]
    P --> C[Agent calls agh__clarify with ≤4 choices]
    C --> Q[SSE question card + CLI/HTTP pending projection]
    Q -->|operator answers choice or free text| AN[Tool result carries the exact answer; turn unblocks]
    Q -->|nobody answers before timeout| TS[Unanswered sentinel Choice=nil Text empty Fallback=true — never a synthesized selection]
    Q -.->|operator closes the tab| AB[Abandon: question stays pending in-memory]
    AB -.->|returns before timeout| AN
    AB -.->|daemon restarts| CLR[Pending cleared per-boot; no ghost prompt]
    AN --> TE[True end: fresh reads show the grant list, zero re-prompts for granted calls, and the clarify answer inside the durable tool result]
    RP --> TE
```

```yaml
journey:
  id: J-answer-agent-requests
  name: "Answer an agent's requests once"
  value_statement: "A decision I already made — approval or answer — is remembered, revocable, and never re-asked; an agent question blocks until I answer instead of dead-ending."
  personas: [Théo, Ada]
  entry_points:
    - url: "web session timeline (approval prompt / clarify question card via SSE)"
      origin: in-app-nav
    - url: "CLI: agh tool approvals set|list|revoke; agh session clarify pending|answer"
      origin: direct
    - url: "HTTP/UDS: /api/tool-approval-grants; /api/workspaces/:workspace_id/sessions/:session_id/clarifications"
      origin: direct
  actions:
    - step: 1
      verb: "Answer a native-tool approval prompt with allow_always"
      expected_observable: "A durable grant persists at the most-specific key (workspace+agent+tool+input_digest); the identical follow-up call runs with zero prompt round-trip"
    - step: 2
      verb: "Restart the daemon and repeat the call"
      expected_observable: "The grant still auto-approves; a non-matching agent or workspace still prompts; deny-all mode still denies"
    - step: 3
      verb: "Set one explicit agent-wide and one tool-wide decision, then revoke"
      expected_observable: "Wider grants exist only through explicit set surfaces (no input_digest), list identically across Web/CLI/HTTP/UDS/native, and revocation restores prompting"
    - step: 4
      verb: "Answer a live agh__clarify question"
      expected_observable: "The card shows the exact question and ≤4 choices; the answer lands in the tool result and unblocks the turn; a timeout resolves the unanswered sentinel"
  goal:
    observable: "Zero re-prompts for remembered decisions; the clarify answer is inside the durable tool result"
    side_effects: [approval-grant-persisted, canonical-grant-events, clarify-session-events]
  true_end_state: "After restart and fresh reads: the grant list matches on every surface, a matching call runs unprompted, a revoked key prompts again, and resolved/timeout clarify receipts remain in the transcript without leaking into another workspace."
  exit:
    natural: "The operator returns to watching the session; the agent continues with the approved tool or the answered question."
  abandonment:
    - at_step: 4
      how: "The operator never answers the clarify question."
      resume: "The configured timeout resolves the explicit unanswered sentinel (never a synthesized choice); pending state is per-boot, so a restart clears it without ghost prompts."
  crosses: [tool-approval-bridge, three-mode-policy, GlobalDB-grants, clarify-broker, SSE, Web-settings, CLI, HTTP, UDS, native-tools]
```
