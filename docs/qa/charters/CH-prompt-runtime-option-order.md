# CH-prompt-runtime-option-order: Apply each prompt runtime atomically and in order

```yaml
charter:
  id: CH-prompt-runtime-option-order
  mission: "As Bruno, change Reasoning, Fast, and typed ACP options at prompt boundaries and prove each accepted snapshot is fully applied in deterministic order or rejected before dispatch."
  mode: strategy-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-21
  scenarios: [RT-061, RT-session-prompt-runtime-transitions]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "On a standard ACP, capture model first, refreshed descriptors second, Reasoning third, Fast fourth, remaining option IDs in stable order, and prompt dispatch last."
      - "Change one value after the first prompt and prove the next prompt owns a new immutable snapshot while earlier evidence stays unchanged."
      - "Force descriptor drift between two responses and prove the next write is revalidated before dispatch."
      - "Force a rejected transition and prove the prior active runtime remains usable with no partially dispatched prompt."
    must_avoid:
      - "Do not infer ordering from UI state alone; require ACP trace and a fresh structured session read."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
