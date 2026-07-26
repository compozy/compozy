# J-07 — Agent-operated run through structured surfaces

The agent-manageability journey (PRD F12, primary persona "Autonomous agent", ADR Agent-Manageability). An ACP agent discovers a Loop, supplies its declared inputs, runs it, and monitors it to a terminal outcome **entirely through structured `agh__loop_*` tool output** — no human, no web UI. This is the "manageable by agents" half of AGH's core premise: every web action has a structured equivalent, output is deterministic, and the capability gates hold.

```mermaid
flowchart TD
    A[Entry: agent calls agh__loop_list / agh__loop_inspect] --> B[Discover the Loop + its declared input schema as structured output]
    B --> C{Service ready?}
    C -->|loop.Service not yet wired| C2[Unavailable ReasonDependencyMissing — deterministic, retryable]
    C -->|ready| D[agh__loop_run with declared inputs — parity with CLI/HTTP/UDS run]
    D --> E[Side effect: loop_run created; agent polls agh__loop_status]
    E --> F[Structured status value IS the state — one of the 11, never coerced]
    F --> G{Human gate reached?}
    G -->|agent tries to approve its OWN gate| H[Rejected: approve capability gate, no self-approval]
    G -->|operator or ANOTHER agent approves| I[Resume → terminal]
    F --> J[True end: agent reads a terminal outcome via structured output; matches operator/HTTP view exactly]
    E -.->|malformed/ambiguous output| X1[Abandon: agent can't parse → the non-determinism IS the bug]
    D -.->|input missing from start[] allowlist| X2[Abandon: rejected with a deterministic ReasonCode, no loop_run created]
```

```yaml
journey:
  id: J-07
  name: "Operate a Loop end-to-end as an autonomous agent via native tools"
  value_statement: "An agent runs and monitors a Loop through structured, non-UI surfaces with deterministic output and enforced capability gates — proving web-UI-only control is never required."
  personas: [Ada]
  entry_points:
    - url: "native tools: agh__loop_list / agh__loop_inspect / agh__loop_run / agh__loop_status / agh__loop_approve"
      origin: in-app-nav
    - url: "CLI: agh loop list|inspect|run|status|approve (UDS + HTTP parity)"
      origin: direct
  actions:
    - step: 1
      verb: "Discover the Loop and its input schema"
      expected_observable: "agh__loop_list / inspect return structured definition + declared-input schema; Unavailable(ReasonDependencyMissing) with a deterministic ReasonCode before the service is ready"
    - step: 2
      verb: "Run the Loop with declared inputs"
      expected_observable: "agh__loop_run creates a loop_run; the native-tool output matches CLI/HTTP/UDS for the same inputs (no positional args)"
    - step: 3
      verb: "Monitor to terminal via structured status"
      expected_observable: "agh__loop_status returns one of the 11 states as the literal value — never inferred, never coerced"
    - step: 4
      verb: "Attempt to approve (capability gate)"
      expected_observable: "The agent cannot approve its OWN gate; an operator or a different agent can"
  goal:
    observable: "The agent drives a Loop from discover → run → terminal purely via structured tools, and the terminal outcome matches the operator/HTTP view exactly"
    side_effects: [loop_run-created, native-tool-audit]
  true_end_state: "The agent's structured terminal outcome equals the operator's (verified by comparing structured agent operation against operator operation — PRD 'Surface coverage'); no self-approval slipped through; token redaction is hash-form-only."
  exit:
    natural: "Agent holds a terminal outcome and its evidence, all via structured output."
  abandonment:
    - at_step: 3
      how: "Structured output is ambiguous or non-parseable."
      resume: "The non-determinism is itself the finding — an agent that can't parse the status can't operate the Loop."
    - at_step: 2
      how: "The agent's start surface is not in the Loop's start[] allowlist."
      resume: "Rejected with a deterministic ReasonCode; no loop_run created (a binding outside the declaration is rejected, not silently dropped)."
  crosses: [native-tool-registry, CLI-HTTP-UDS-parity, capability-gate, agent-status-contract, token-redaction]

design_reference:
  screens: []
  design_note: "No product screen — this journey is the structured/non-UI surface (F12). The 'truthful UI' contract still applies to structured output: the status VALUE is the state. Parity is checked against the operator view (run-detail §4.4 / runs.html §4.5) that renders the same underlying run."
  truthful_ui_checks:
    - "The structured status value equals one of the 11 states and is never coerced (exhausted/stalled/needs-approval never returned as done/failed) — ADR-013 inv5."
    - "approve capability gate: an agent cannot approve its own gate; hash-form-only token redaction in tool output (N-005)."
    - "Every web action (run/configure/edit/pause/resume/stop/approve) has a matching structured verb — web-UI-only control would be incomplete (F12)."

e2e_backbone:
  runtime:
    - "E2E-runtime-3: CLI↔HTTP↔UDS parity across run/dry-run/configure/pause/resume/stop/approve/list/inspect/status/runs/edit/delete."
    - "E2E-runtime-5: start a loop from every surface (incl. agent native tool) and reach an identical terminal outcome (ADR-007)."
    - "E2E-runtime-8: an agent cannot approve its own gate; operator/another agent can (N-005)."
  integration:
    - "Integration-27: match agh__loop_* tool output against HTTP/UDS state for the full verb set; enforce the approve capability gate with no self-approval; hash-form-only token redaction."
    - "Integration-28: return Unavailable(ReasonDependencyMissing) until loop.Service is ready, with deterministic ReasonCode contracts."
  followups:
    - "AB-003 — a CLI↔HTTP↔UDS↔native-tool full-verb parity harness for the agent surface (E2E-runtime-3 / Integration-27) is the highest-value automation; flag for a dedicated parity fixture."
```
