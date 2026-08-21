# CH-profile-palette-lens-isolation: Run two palettes on two profiles and find the seam that shares state

```yaml
charter:
  id: CH-profile-palette-lens-isolation
  mission: "As Bruno, keep two clients open on two profiles and drive the palette in both at once — catalog, search, domain views, view sessions, invoke, ranking, pins, and extension contributions — hunting for the one cache, key, revision, or history that carries something from one lens into the other, and proving every lifecycle action hands off to the canonical dialog instead of mutating on its own."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-command-profiles-from-palette
  scenarios: [ET-profile-palette-view, ET-profile-palette-lens-isolation, ET-palette-domain-views, ET-palette-registry-driven-root, ET-palette-sessions-view-switch, ET-extension-palette-contributions, ET-agent-palette-config-parity]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Build divergent histories: in each client use different commands, type different queries, and pin different rows. Then compare ranking, recents, and pins per profile, switch the aggregate lens on, and prove it keeps its own history and can neither read nor mutate a real profile's."
      - "Open the Profiles view in both clients: confirm current, archived, needs-setup, and a disabled unavailable row carrying the runtime's own reason; run `profile.use` in one client and prove the other is not force-switched; start each lifecycle action and prove it opens the canonical dialog with its plan revision, refuses a stale plan, and still gates delete behind its confirmation."
      - "Switch profiles with a domain view open and with a view session live — the view, its cached rows, and the SSE invalidation must all re-fence, and a browser reload must not resurrect a row from the previous lens."
      - "Disable an extension in one profile only and prove its commands, views, aliases, and default chords vanish there, stay live in the other, change the catalog revision, and that a user-authored override on a contributed command survives dormant and reactivates on re-enable. Write one `[cmd_palette]` key under `--scope profile` and prove the effective value differs per profile while a `--global` binding is refused from a profile layer."
    must_avoid:
      - "The generic palette grammar and root behaviors the existing CH-palette-* charters own; scoped work reads outside the palette (CH-profile-foreign-leak-probe); default COMPOZY_HOME or ports — use the isolated lab from the bootstrap manifest."
```

## Selection rationale

ADR-016 is the newest and least-walked part of the program: it adds a required scope dimension to
every palette transport and cache key, rebuilds three personalization tables around
`profile_lens_id` with `@all` reserved, and filters extension contributions by enablement and
placement before the catalog revision is computed. Safety Invariant 20 states the whole contract in
one line — every catalog, search, view-session, invoke, event, cache, and personalization identity
carries one real profile or the labeled aggregate. The spec's own risk register names web cache
bleed as a known risk, and the failure it predicts is a briefly stale row after a switch. Two live
clients on two profiles is the only setup that exposes it, which is why this is a Multi-Tab Tour
rather than a feature walk.

## Evidence and entry points

- **Web** — side-by-side screenshots of both palettes at rest, the Profiles view with the current, archived, needs-setup, and disabled rows, the canonical dialog opened by handoff, and the same views after a switch and after a reload.
- **CLI** — `cmd-palette list`, `cmd-palette list --source ext.<name>`, `bindings -o json`, and `personalization show` per lens; the denylist refusal for the `--global` binding from a profile layer.
- **HTTP and UDS** — catalog, search, view, view-session, and invoke calls with no profile parameter (must resolve `default`), with both a profile and the aggregate (must conflict), and the enablement write that changes the catalog revision.
- **Agent** — `compozy__cmd_palette_list` from inside a session showing the session-derived lens, and the refusal when a different acting profile is supplied.
- **Runtime** — personalization row counts per lens before and after a profile delete.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
