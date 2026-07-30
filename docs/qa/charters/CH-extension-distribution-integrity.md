# CH-extension-distribution-integrity: Abuse every source while preserving trust and progress truth

```yaml
charter:
  id: CH-extension-distribution-integrity
  mission: "As Ada, publish one release and abuse curated, GitHub, git, and local_path acquisition, proving one-command consent, pre-mutation integrity, passive update discovery, deterministic remediation, and durable per-item progress through update and removal."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: flaky
    locale: en-US
  journey: J-extension-distribution
  scenarios: [ET-extension-publish-install-round-trip, ET-extension-published-source-installs, ET-web-extension-union-install, ET-extension-passive-update-discovery, ET-extension-batch-update-partial-progress, ET-extension-cli-error-remediation, ET-015, ET-017, ET-018, ET-019, ET-020, ET-023]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Publish deterministic archive and SHA-256 sidecar artifacts, then consume them from a second isolated home. Scan tool input/output, CLI, events, logs, and release diagnostics for the server-resolved credential."
      - "Submit each generated source-union variant over CLI, HTTP, UDS, native, and Web where supported. Own-source installation must take one command and at most one explicit consent; invalid source/ref combinations and absent consent must write nothing."
      - "Tamper with a curated archive and a GitHub sidecar. Curated verification alone may set checksum_verified; a matching sidecar remains integrity-only, and any mismatch aborts before registry state with no override."
      - "Publish a behavior-changing version, prove list/search/Web advertise it without a check flag, then degrade one source and partially fail update --all. Local inventory remains available and every successful item stays committed with ordered outcomes and events."
    must_avoid:
      - "Editing registry rows, using catalog submission as a release step, treating a GitHub sidecar as trust, polling with a hidden update-check flag, or collapsing a partial batch into all-or-nothing."
  evidence_expectations:
    - "Source-union request/response matrix across supported planes, command/consent count, pre/post failure registry and filesystem reads, and provenance comparisons."
    - "Passive update captures from list/search/Web, degraded-source marker, ordered batch outcomes/events, changed invocation result, and final removal reads."
```

## Selection rationale

Targeted tier. ADR-005 owns GitHub/git distribution and ADR-007 owns its source-policy controls.
Safety Invariants 4–5, 12–13 are the integrity and credential hot spots. This charter owns the
`own install ≤ 1 command + 1 consent` and passive-update scorecard measures.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
