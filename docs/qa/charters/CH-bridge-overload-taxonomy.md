# CH-bridge-overload-taxonomy: Provider overload recovers once, correctly classified, never replayed

```yaml
charter:
  id: CH-bridge-overload-taxonomy
  mission: "As Omar, throttle and fail a fake first-party bridge provider to prove the 529/500/reset taxonomy, the single bounded overload retry, preserved Retry-After, and zero replay after a committed mutation."
  mode: charter-with-tour
  persona:
    name: Omar
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-connect-bridge-provider
  scenarios: [NB-bridge-overload-recovery]
  tour: Network Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Fake provider returns 529 then success → classified overloaded, one bounded wait through the distinct overload profile, then success; 500 → server_error; connection reset → transient."
      - "Positive Retry-After preserved exactly in the wait; a committed remote mutation is never replayed."
      - "Confirm no provider-local retry loop remains and delegated ACP agent paths show zero behavior diff."
    must_avoid:
      - "Real provider credentials; unbounded retry storms against the fake provider."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
