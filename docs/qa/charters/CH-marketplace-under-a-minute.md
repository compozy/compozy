# CH-marketplace-under-a-minute: The mid-session operator acquires all four kinds, each under a minute

```yaml
charter:
  id: CH-marketplace-under-a-minute
  mission: "As Bruno, mid-session, detour to Marketplace and acquire one capability of each kind (skill, extension, bundle, MCP) born-valid — capturing per-kind wall-clock from opening Marketplace to installed/activated, each under 60 seconds — and prove discovery, management, and update states agree afterwards."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-marketplace-acquisition
  scenarios: [ET-web-marketplace-landing-browse, ET-web-marketplace-search-fanout, ET-web-marketplace-skill-install, ET-web-mcp-guided-install, ET-web-bundle-preview-activate, ET-web-bundle-activation-detail, ET-web-ext-policy-block, ET-web-catalog-navigation, ET-web-extensions-manage, ET-web-extension-detail, ET-010, ET-014]
  tour: Money Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Start a timer when the Marketplace sidebar item is clicked and stop it at the kind's true end (skill: installed + Manage lands on /skills/$name; extension: installed with truthful provenance; bundle: activation visible after preview; MCP: structurally valid server with truthful readiness on /mcp). Record all four timings — the PRD anchor is <60s per kind."
      - "Search once and break one kind's source (isolated feed): the failed section owns its error strip while the other kinds stay usable; an all-zero query offers one clear-search recovery."
      - "Attempt the unverified extension under default policy: Install must be focusable-but-unavailable, explain the real warning, link Settings › Extensions, and write nothing. Do the keyboard walk knowing BUG-20260714-keyboard-focus-invisible is open — record whether focus is findable, don't work around it."
      - "In the MCP guided modal, submit with a missing required value and with a dangling vault ref: both must block with nothing written; then complete with one typed and one vault-referenced secret and confirm no plaintext in network, DOM, or fresh settings reads."
      - "After each acquisition, re-read the marketplace and the kind's management home on a fresh load: installed state, version, provenance, and update badges must agree (skills/extensions semver only, bundle spec_drift only, MCP badge-less)."
      - "Verify the sidebar Catalog order is exactly Marketplace · Extensions · Bridges · Skills · MCP · Knowledge, and /skills is installed-only with the Browse Marketplace recovery in topbar and empty state."
    must_avoid:
      - "Authorizing the MCP server (CH-mcp-authorize-repair-truth owns it); changing extension policy (CH-extension-policy-admin-gates owns Settings › Extensions); CLI/API acquisition (CH-agent-marketplace-parity)."
  evidence_expectations:
    - "A per-kind timing table (start/stop timestamps, elapsed seconds, pass/fail vs 60s) in the run report — this is the PRD time-to-acquire anchor's capture."
    - "Screenshots at each kind's true end state and at the partial-failure strip; fresh-read JSON or screenshot proving discovery/management agreement per kind."
    - "For every blocked install attempt: proof nothing was written (fresh settings/extensions/skills reads)."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
