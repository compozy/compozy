# CH-palette-catalog-revision-truth: Prove the palette never lies when the daemon is cold, slow, or gone

```yaml
charter:
  id: CH-palette-catalog-revision-truth
  mission: "As Bruno on a throttled and interrupted connection, open, search, and execute from the palette while the daemon degrades — proving the last-known catalog renders instantly, availability degrades to disabled-with-verbatim-reason (never allow-all, never blank), async entity waves and the fallback row assemble honestly, and recovery converges every surface on one revision."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-command-os-from-palette
  scenarios: [ET-palette-registry-driven-root, ET-window-tab-palette-search, ET-palette-agent-fallback]
  tour: Network Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Stop the daemon with the palette open and reopen it cold — the last-known catalog renders instantly, action rows disable with 'runtime unavailable', exempt commands keep working, and recovery re-enables rows without reopening; no blank or blocked palette at any point."
      - "Throttle the connection and search entities — sections fill as each domain resolves without reordering or stealing selection; kill one domain endpoint — only that section shows an inline error naming the domain."
      - "Type a zero-match and a weak-match query under throttling — the Ask-agent row assembles per the served threshold, nothing transmits before ⏎, and a spawn failure preserves the query with the reason named; verify a tab result at the threshold boundary coexists with the fallback row."
      - "Enable an extension from a second client mid-search — the open palette converges on the new catalog revision within the invalidation window, with no partial merge (rows and their chords change together or not at all)."
    must_avoid:
      - "Approval flows (CH-palette-approval-exactly-once); pretending a Storybook or unit pass is a walk — this charter exists because the projection's honesty only shows against a real degrading daemon."
```

## Selection rationale

Targeted tier: SI-3 (revision-consistent renders, no partial merges), BR-19 (never block on a cold
daemon), BR-8 (verbatim reasons everywhere), and BR-12 (nothing pre-sends) are the structural-
revision hot spots named by the task. ADR-001's unified registry and ADR-006's daemon-canonical web
projection meet in the IndexedDB stale-while-revalidate cache and SSE invalidation paths shipped in
tasks 02–03. The Network lens is their native habitat.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
