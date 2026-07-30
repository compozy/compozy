# CH-prompt-runtime-fail-loud: Reject unusable runtime choices before prompt dispatch

```yaml
charter:
  id: CH-prompt-runtime-fail-loud
  mission: "As Ada, abuse the prompt-runtime boundary through HTTP, UDS, and CLI and prove unavailable models or reasoning choices fail with one deterministic code before changing the active session."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-21
  scenarios: [MS-057, RT-062]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Against the same live session, request an unavailable model, a model with no reasoning option, and an effort outside the advertised subset through HTTP, UDS, and compozy session prompt."
      - "For every rejected request, capture the structured 422 body and CLI output; the code must be model_unavailable, reasoning_option_missing, or reasoning_effort_unsupported consistently across surfaces."
      - "Fresh-read the session and ACP trace after each failure to prove no rejected prompt dispatched and the prior active runtime remains intact; send one valid follow-up prompt to demonstrate recovery."
      - "Submit one malformed off-contract effort at the public boundary and confirm it is rejected without coercion, partial queue entry, or silent provider-default substitution."
    must_avoid:
      - "Do not test catalog-curation model_not_found here; it belongs to J-20 and CH-031."
      - "Do not treat a server log, unit test, or optimistic UI state as proof without the independent public read and ACP no-dispatch evidence."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
