# CH-extension-newcomer-first-success: A newcomer follows the public quickstart to working code

```yaml
charter:
  id: CH-extension-newcomer-first-success
  mission: "As Lea, outside the Compozy repository on a release-stamped binary, follow the public quickstart verbatim and prove the first working extension arrives within ten concepts and four actions, with no hidden manifest, trust, or repository-only step."
  mode: charter-with-tour
  persona:
    name: Lea
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-newcomer-first-success
  scenarios: [ET-extension-quickstart-verbatim, ET-extension-dx-scorecard]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Use a release-stamped binary from outside the checkout. Copy only the fenced quickstart commands from the matching public site and record each introduced concept and user action until the first successful invocation; fail the scorecard above ten concepts or four actions."
      - "Inspect the scaffold for public dependencies only, then run the dev lane without creating or editing a manifest and without changing trust configuration or accepting a marketplace prompt."
      - "Edit the handler, wait for watch, and invoke again. The changed result must appear without reinstalling; a validation failure must name a source-level remediation and leave the prior behavior callable."
      - "Re-grade SDK simplicity from the same newcomer artifacts, counting generated or required ceremony exactly as the brief rubric does rather than discounting it as boilerplate."
    must_avoid:
      - "Repository-local replace directives, unpublished examples, dev builds that bypass the release version gate, shell commands not printed by the guide, and operator hints that conceal a missing doc step."
  evidence_expectations:
    - "Release version, external working directory, copied quickstart commands, action/concept ledger, first and changed invocation results, and any rendered remediation."
    - "A scorecard row for concepts ≤10 and first-success actions ≤4, plus the simplicity-grade inputs used by the unchanged rubric."
```

## Selection rationale

Targeted tier. ADR-001 and ADR-002 own the code-first and dev-lane promises; ADR-003 owns the
release-matched public SDK contract. Safety Invariants 10–12 guard generated drift, deterministic
manifest output, and author-only describe execution. This is the required newcomer-outside-repo
journey and the primary scorecard acceptance box.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
