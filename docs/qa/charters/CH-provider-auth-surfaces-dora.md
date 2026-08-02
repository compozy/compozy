# CH-provider-auth-surfaces-dora: Verify provider auth without disclosure

```yaml
charter:
  id: CH-provider-auth-surfaces-dora
  mission: "As Dora, configure and operate provider authentication through its public surfaces while proving every read path stays safe and truthful."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-provider-auth
  scenarios: [RT-025, RT-026, RT-027]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Write one login command through config, then inspect CLI, HTTP, UDS, native config readback, provider status/doctor, and Web Settings after refresh."
      - "Probe an auth-mode-none provider, a provider with no status command, an unknown provider, and one isolated executable in a controlled final environment and working directory."
      - "Use a command containing arguments, an environment prefix, and a distinctive path; verify that only its basename and safe descriptor fields can be read back."
      - "Run the CLI-only login action with supported no-TTY and timeout controls, then confirm HTTP and UDS have no login route."
      - "Attempt empty, whitespace-bearing, case-varied, and stale provider inputs only through documented public surfaces; capture the recovery guidance."
    must_avoid:
      - "Do not read source, query SQLite, inspect private config through the filesystem, or use an internal endpoint to decide a verdict."
      - "Do not treat a successful mutation response as proof until fresh independent reads agree."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
