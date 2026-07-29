# CH-extension-dev-recovery: Interrupt reloads and prove the last-good workspace instance survives

```yaml
charter:
  id: CH-extension-dev-recovery
  mission: "As Bruno, interrupt and corrupt the workspace dev loop at generation, activation, origin, and log-transport boundaries, proving serialized immutable swaps, last-good recovery, bounded redacted logs, and strict workspace isolation."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: flaky
    locale: en-US
  journey: J-extension-dev-lifecycle
  scenarios: [ET-extension-code-first-authoring, ET-extension-dev-reload-loop, ET-web-extension-logs-panel, ET-022]
  tour: Interrupt Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Create workspace A's dev link through CLI, HTTP, UDS, and native reads; inspect request bodies/tool inputs to prove workspace_id is never accepted and the global published instance remains distinct."
      - "Race watch with explicit reload, interrupt a build, submit a stale or malformed generation hash, and fail activation after validation. No partial generation may become observable; the last-good generation must run with explicit activation_failed status."
      - "Move or escape the origin after linking. Canonical containment must be rechecked, status must become missing_origin, and no binary outside the recorded generation directory may execute."
      - "Emit configured secrets and more than 256 KiB of stderr, then follow over CLI, HTTP/UDS named extension_log SSE, native logs, and the Web panel. Oldest lines may drop; ingestion must already be redacted, the producer must not block, and reconnect must remain monotonic."
    must_avoid:
      - "Publishing or global-install trust cases, direct database edits, reading global logs through an agent scope, or accepting an unnamed SSE message event as equivalent."
  evidence_expectations:
    - "Per-instance state captures for workspaces A/B and global, operation ordering, immutable generation handles, last-good invocation, and missing-origin refusal."
    - "Ring bound and redaction evidence upstream of CLI/HTTP/UDS/native/Web, including reconnect sequence and teardown of every follower."
```

## Selection rationale

Targeted tier. ADR-001–ADR-002 define the generation and dev-link model. Safety Invariants 1–3,
7–8, 12, and 14–15 are concentrated here; they are the highest-blast-radius runtime and isolation
hot spots in the program.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
