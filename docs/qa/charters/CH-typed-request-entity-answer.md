# CH-typed-request-entity-answer: Resolve annotated entity answers exactly once

```yaml
charter:
  id: CH-typed-request-entity-answer
  mission: "As Bruno, answer one nested entity-annotated request and prove invalid references stay recoverable without resuming work."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: flaky
    locale: en-US
  journey: J-supervise-loop-request
  scenarios: [LP-answer-typed-request-entities]
  tour: Network Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Open the same nested x-compozy-kind request through Web, CLI, HTTP/UDS, and native detail reads."
      - "Submit a missing exact entity, refresh, and confirm the request remains pending with the nested field error."
      - "Submit a valid entity once, retry it, and confirm one durable winner and one resume."
      - "Compare enum precedence and nested array/object controls without exposing secret values."
    must_avoid:
      - "Optimistic UI or direct database reads as proof."
      - "Treating request_validation_failed and input_validation as interchangeable."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
