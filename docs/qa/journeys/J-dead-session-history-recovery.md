# J-dead-session-history-recovery — Read and fork a dead session

A returning session user opens a session after its ACP provider process ended unexpectedly and all
three automatic recovery attempts failed. The session must remain a truthful record of the bounded
recovery, not an endless restart loop. The user reads the preserved transcript and diagnostics, then
forks a new child only if they choose to continue.

```mermaid
flowchart TD
    E[Entry: active turn or session deep link] --> A[Three automatic recovery attempts fail]
    A --> X[One recovery-exhausted event and one terminal failure]
    X --> H[Read persisted transcript and diagnostics]
    H --> P{Is the runtime terminal process_exit?}
    P -->|yes| RO[Show read-only history and disable prompt]
    RO --> R[Refresh and read the same transcript again]
    R --> F[Fork into a new session]
    F --> C[Open child session with parent provenance]
    RO -.->|leave now| L[Original history remains available]
    C --> T[True end: original remains intact; child is ready for new work]
```

```yaml
journey:
  id: J-dead-session-history-recovery
  name: "Read and fork a dead session"
  value_statement: "I can understand a session after bounded automatic recovery is exhausted without losing its history or entering an endless retry loop."
  personas: [Théo]
  entry_points:
    - url: "web session deep link or session list"
      origin: return-visit
    - url: "CLI: compozy session recap <session-id>"
      origin: direct
  actions:
    - step: 1
      verb: "Wait for the bounded provider recovery to exhaust"
      expected_observable: "Three recovery attempts are recorded before one terminal process_exit failure; partial output remains visible."
    - step: 2
      verb: "Refresh and re-read the stopped session"
      expected_observable: "The same transcript remains available and no session/load retry or generic server error appears."
    - step: 3
      verb: "Fork into a new session"
      expected_observable: "A child opens in the same workspace with parent_session_id pointing to the original; the original remains unchanged and readable."
  goal:
    observable: "The user can safely inspect the original failure and start separate follow-up work without rewriting history."
    side_effects: [child-session-created]
  true_end_state: "The original transcript, recovery history, and diagnostics survive refresh; no fourth attempt starts; the child is a distinct session linked to its parent."
  exit:
    natural: "The user either leaves with the original history intact or continues in the child session."
  abandonment:
    - at_step: 1
      how: "The user closes the tab after seeing the runtime unavailable notice."
      resume: "Reopen the same deep link; persisted history remains readable without attaching to ACP."
  crosses: [web, CLI, HTTP, UDS, session-store, transcript-projection, workspace-isolation]
```
