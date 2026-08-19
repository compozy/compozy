# CH-loop-request-lifecycle: Resolve one request exactly once across every surface

```yaml
charter:
  id: CH-loop-request-lifecycle
  mission: "As Bruno on a flaky connection, find, inspect, and resolve ask and review requests while proving one winner, exact expiry/cancel behavior, and honest attention counts."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: flaky
    locale: en-US
  journey: J-supervise-loop-request
  scenarios: [LP-ask-answer, LP-request-expiry, LP-review-edit-execute, LP-web-request-answer-card, LP-web-bell-loop-requests, LP-web-timeline-graph-rows]
  tour: Network Tour
  time_box_minutes: 60
  invariants: [Safety 1 one answer wins, Safety 3 reviewed action executes once, Safety 12 expiry once, Safety 13 cancel is atomic]
  guidance:
    must_try:
      - "Compare request list/detail/respond through CLI JSON, HTTP, UDS, native tools, and fresh Web reads."
      - "Submit an invalid answer, drop the network during a valid answer, and race a duplicate response."
      - "Exercise authored expiry and loops.defaults.<kind>.requests.expire_after, including restart near the deadline."
      - "Open the request from the attention bell, switch workspace, then prove resolution drains the bell and run-list count."
    must_avoid:
      - "Database reads, optimistic UI as proof, or a mock-only request seed."
```

