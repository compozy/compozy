# J-supervise-loop-request — Resolve a Loop request and resume trusted work

```mermaid
flowchart TD
    A[Entry: attention bell, run page, CLI, HTTP/UDS, or native tool] --> B[Start a Loop that reaches ask or review]
    B --> C[Request opens and the run parks]
    C --> D[Find the pending request and read its full redacted detail]
    D --> E{Responder action}
    E -->|valid answer or decision| F[One response wins and the exact parked cell resumes]
    E -->|invalid payload| G[Field error; request remains pending]
    E -->|self-operation or missing capability| H[Deterministic denial; no state changes]
    C -->|expiry or cancellation wins| I[Request closes once and its declared outcome runs]
    F --> J[Fresh CLI, HTTP/UDS, native, SSE, and Web reads agree]
    I --> J
    J --> K[True end: request is absent from live attention and run history names the outcome]
    C -.->|operator leaves| X1[Abandon: request stays durable and resumes after restart]
    D -.->|network drops during submit| X2[Resume: refresh shows either one winner or the still-actionable request]
```

```yaml
journey:
  id: J-supervise-loop-request
  name: "Resolve a Loop request and resume trusted work"
  value_statement: "An operator or permitted agent can find one durable human request, act once, and trust every public surface to show the same outcome."
  personas: [Bruno, Ada]
  entry_points:
    - url: "web attention bell and /loop-runs/:runId Needs-you card"
      origin: in-app-nav
    - url: "CLI: compozy loop requests|request|respond"
      origin: direct
    - url: "HTTP/UDS Loop request routes"
      origin: direct
    - url: "native tools: compozy__loop_requests|request|respond"
      origin: direct
  actions:
    - step: 1
      verb: "Open a request-bearing Loop and let it park"
      expected_observable: "The request appears once with a bounded preview and exact pending aggregate."
    - step: 2
      verb: "Read the request through a public detail surface"
      expected_observable: "The full redacted context, schema, decisions, expiry, and provenance are reachable inside the workspace."
    - step: 3
      verb: "Submit an answer or review decision"
      expected_observable: "One schema-valid response wins; invalid, duplicate, expired, canceled, unauthorized, and self responses return deterministic errors without corrupting state."
    - step: 4
      verb: "Refresh and compare every public read"
      expected_observable: "The run resumes or closes by the recorded cause, the request drains from live attention, and structured surfaces agree."
  goal:
    observable: "The exact request is resolved once and the owning run continues from the admitted payload or decision."
    side_effects: [request-ledger, coordinator-resume, attention-drain, status-and-event-updates]
  true_end_state: "A fresh browser load and independent CLI/API/native reads show the same winner, provenance, run state, and zero live count for the closed request."
  exit:
    natural: "Operator lands on the resumed run and can read the decision in history."
  abandonment:
    - at_step: 2
      how: "The operator leaves before answering."
      resume: "The request remains pending across daemon restart until answered, expired, or canceled."
    - at_step: 3
      how: "The network drops while the response is in flight."
      resume: "A fresh read reveals one committed winner or a still-actionable request; resubmission cannot execute the action twice."
  crosses: [request-store, scheduler-expiry, config-lifecycle, CLI, HTTP, UDS, native-tools, SSE, web-attention]
```

