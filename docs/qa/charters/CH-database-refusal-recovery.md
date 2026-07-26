# CH-database-refusal-recovery: Refuse incompatible databases without silent mutation

```yaml
charter:
  id: CH-database-refusal-recovery
  mission: "As Bruno, confront AGH with pre-Goose and ahead databases, preserve incompatible state, and prove daemon and stopped-daemon CLI opens fail deterministically before normal work."
  mode: strategy-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-daemon-schema
  scenarios: [RT-refuse-legacy-database, RT-refuse-legacy-cli-open, RT-refuse-ahead-database]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Seed the legacy and ahead fixtures before startup, record each agh.db digest, then run `agh daemon start --foreground`; require non-zero exit before readiness and an unchanged digest."
      - "For the legacy home, require the canonical database path plus whole-family preserve-or-move/fresh-start remediation; stop AGH, move the complete AGH_HOME or workspace .agh directory with every sibling database, select a separate fresh home, and confirm restart succeeds."
      - "With the daemon stopped on the legacy home, run `agh extension list -o json` and `agh mcp auth status -o json`; configure a bound-secret provider and run `agh provider auth status <provider> -o json` when the fixture supports it."
      - "For the ahead fixture, require the newer-binary-or-whole-family-preservation remediation from daemon startup and at least one local direct-open command."
    must_avoid:
      - "Editing migration history to repair a refused database; fixtures are prepared before the session and incompatible homes stay preserved."
      - "External OAuth or native-provider login; the refusal must happen before those dependencies are consulted."
```

<!-- The charter is durable: each run's debrief belongs in its dated report. Safety-critical recovery guidance may be corrected in place. -->
