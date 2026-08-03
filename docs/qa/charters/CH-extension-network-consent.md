# CH-extension-network-consent: Accept only the exact current Network requirement

```yaml
charter:
  id: CH-extension-network-consent
  mission: "As Bruno, race and replay extension enable, update, and dev reload around changing Network digests, proving only exact current consent commits and every refused operation leaves the previous persisted and running state intact."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-kit-lifecycle
  scenarios: [ET-ext-network-confirm, ET-019]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Use the cycle bootstrap manifest and one serialized config authority; open the installed extension in two browser tabs while CLI, HTTP, UDS, and native calls share the same isolated daemon state."
      - "Preview the current digest, refuse enable without it, retry with a stale digest, then confirm the exact digest through operator and agent lanes. The 409 always carries the current digest; confirmed_by distinguishes operator from agent:<session-id>."
      - "Stage a managed update whose requirement changed. Race stale and current retries from two tabs or callers; before exact consent, registry row, installed files, confirmation tuple, live owner set, scheduler, and running generation remain byte-equivalent to the old version."
      - "Run update --all with one digest-changing item: completed siblings stay explicit, the changed item is refused with its digest, and no batch field can blanket-confirm it. Retry that item alone with the exact digest."
      - "Change a dev-linked generation's digest and reload global, workspace A, and workspace B instances. Each confirmation stays instance-scoped, and a dev-only instance persists consent on its link without borrowing the global row."
      - "Restart the daemon and compare browser, CLI, HTTP, UDS, native, events, and inventory. Confirmation survives only for the matching digest, and no requirement declaration enrolls an execution into Live."
    must_avoid:
      - "Parallel config writes, bare boolean consent, accepting a digest copied from another instance, or treating the Network manifest block as channel enrollment."
```

## Evidence expectations

- Before/refusal/after byte comparisons; two-tab screenshots; actor and instance confirmation rows;
  batch partial result; dev-link reload evidence; restart parity; clean event payloads.

## Selection rationale

Targeted tier owner for Safety Invariants 8, 16, and 17 and ADR-005.

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
