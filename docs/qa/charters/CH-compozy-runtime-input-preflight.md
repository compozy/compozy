# CH-compozy-runtime-input-preflight: Defaults and runtime errors fail before ACP spawn

```yaml
charter:
  id: CH-compozy-runtime-input-preflight
  mission: "As Ada, prove input defaults and runtime validation resolve identically across structured surfaces, preserve explicit zero values, and reject invalid execution input before any run or ACP session exists."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-02
  scenarios: [LP-loop-input-defaults, LP-runtime-validation-preflight]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Set global then workspace loops.inputs values sequentially through CLI and HTTP/UDS/native config surfaces; include explicit false, zero, and valid empty values."
      - "Dry-run with and without a per-run value and compare run > workspace > global > definition values and origins across CLI, HTTP, UDS, and native output."
      - "Trigger an unknown input key, type mismatch, authoritative unknown model, non-authoritative arbitrary model, missing bound secret, and incompatible pinned binding."
      - "After every rejected path, prove no loop run, task run, ACP session, budget use, or deep link exists and that another workspace sees only its own/default values."
    must_avoid:
      - "Parallel config writes against the same isolated home."
      - "Treating static definition validation as proof of effective workspace/run validation."
  coverage:
    tier: targeted
    surfaces: [config.toml, CLI, HTTP, UDS, native-config-tools, loop-dry-run, provider-catalog, ACP-boundary]
    invariants: [3, 4, 13, 14]
    adrs: [ADR-001, ADR-002, ADR-007]
    expected_evidence: "Value/origin matrices, typed diagnostics, provider-authority branches, and zero-side-effect storage/session checks."
    exit_criteria: "Every surface resolves the same present values and every invalid path fails before ACP spawn or persistence."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->

