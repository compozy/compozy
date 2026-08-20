# CH-palette-membership-vs-health: Prove extension health degrades availability while only enablement changes membership

```yaml
charter:
  id: CH-palette-membership-vs-health
  mission: "As Bruno running the Go notes fixture through enable, crash-loop, recovery, dev overlay, broken reload, and disable, prove palette membership moves only on enable/disable/remove while health only flips availability — with last-known descriptors, verbatim reasons, dormant defaults, and operator overrides surviving every transition."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-dev-lifecycle
  scenarios: [ET-extension-palette-contributions]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Kill the fixture subprocess mid-use — its commands STAY listed, disabled with the crash-loop reason verbatim in ⌘K, `cmd-palette list`, and Settings > Extensions > Palette; recovery restores availability under a new catalog revision."
      - "Enable with a default chord that conflicts with a core binding — the default stays dormant and visible with its owner named; bind a user override on the fixture's command, disable the extension, re-enable — the override survives dormant and reactivates."
      - "Interrupt a dev reload: one valid edit (projection updates live, open view shows the reloaded note) and one broken manifest (last-good stays live, the error reaches dev diagnostics, nothing half-applies)."
      - "Disable while its declarative view is open — the frame degrades honestly naming the extension, membership drops atomically in one revision step, and Esc/pop keep working."
      - "In the isolated fixture copy, attempt a duplicate ext.* id, an invalid declarative payload, a mutating Tier-1 source tool, and a program declaration without view.provider — each validation failure names the exact field or collision, preserves the incumbent catalog, and never half-registers a contribution."
    must_avoid:
      - "Two-client session racing (CH-palette-view-session-isolation owns it); editing fixture code beyond the manifest."
```

## Selection rationale

Targeted tier: BR-4/SI-5 (membership by enablement, availability by health, atomic revisions) and
SI-11 (duplicate ids reject the later registration without corrupting the incumbent) are the
contribution model's central honesty rules. ADR-002's three contribution surfaces and ADR-009's
two-tier contract meet here; the failure mode (contributions vanishing on a crash or a broken
reload half-applying) silently rewrites the operator's catalog.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
