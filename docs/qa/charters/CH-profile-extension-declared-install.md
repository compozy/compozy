# CH-profile-extension-declared-install: Install a declared context, then try to make the extension undo my decisions

```yaml
charter:
  id: CH-profile-extension-declared-install
  mission: "As Bruno, install an extension that declares a profile, places resources into it, and asks for credentials — then push the whole extension lifecycle at it (update, restart, disable, re-enable, uninstall, reinstall) plus operator archive and delete, to prove creation happens exactly once, an existing name is bound and never seeded, and no extension path ever reverses a lifecycle decision the operator made."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-adopt-extension-profiles
  scenarios: [ET-declared-profile-install, ET-extension-profile-enablement, ET-dormant-extension-placement, ET-ext-kit-enable, ET-web-extensions-manage, NB-notification-preset-profile-enablement]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Read the install preview and confirm it names every declared profile as create or bind, its credential asks, and each resource's placement — and that reading it changes nothing. Confirm, then prove the profile was created without being activated and that needs-setup names the exact vault path that clears it and survives restart, update, and uninstall until the secret is set."
      - "Run the bind path deliberately: create the declared name manually first, install, and prove no identity, defaults, or credential seeding was applied to the operator's profile. Then archive and delete a declared profile and prove boot, update, enable, disable, and repair never resurrect it — and that a full uninstall plus fresh install is treated as a new installation."
      - "Adopt a dormant placement: install a kit placing a resource into an absent profile, confirm it is hidden from the active catalog and shown as dormant with a create action in both detail and preview, then create the name and confirm it publishes there and nowhere else."
      - "Split the machine from the profile: disable the extension in one profile and prove the others keep it, that exactly one exception row exists, and that CLI, HTTP, UDS, native tools, web detail, inventory, and the palette all report the same effective state. Do the same for a notification preset through `compozy notification-preset disable` and prove delivery skips it only there."
    must_avoid:
      - "The palette-side revision invalidation detail (CH-profile-palette-lens-isolation owns it); config and vault layering mechanics (CH-profile-layer-shadow-truth); default COMPOZY_HOME or ports — use the isolated lab from the bootstrap manifest, and drive the install from a mock kit rather than a live registry."
```

## Selection rationale

ADR-002 is the decision with the most ways to go quietly wrong. It grants an extension the power to
create a profile at install with no button and no suggestion step, and then constrains that power
with four safeguards the operator explicitly demanded: create-once per installed instance and name,
bind-never-seed on an existing name, no mutation on update, and no resurrection after operator
archive or delete. Safety Invariant 8 restates all four. A lost or desynced marker produces a
duplicate creation; an over-eager reconcile produces the resurrection the operator rejected. Neither
shows up on a single install — only across the full lifecycle, which is what this session walks.
Per-profile enablement rides along because it is the same shape: a shared library plus exception
rows where absent means enabled.

## Evidence and entry points

- **Web** — the pre-install summary naming the declared profiles, the post-install needs-setup signal, extension detail showing placement per resource and the dormant hint, the per-profile enablement control, and the uninstall copy stating the profile stays.
- **CLI** — `extension install|enable|disable|list`, `profile list` showing the needs-setup flag, `notification-preset list|enable|disable` per profile, and `profile create` for the adoption step.
- **HTTP and UDS** — `POST /api/extensions/preview-install` before and after, `GET|PUT /api/extensions/{name}/enablement`, `GET /api/profiles/{name}` listing each credential requirement with its missing or filled status, and the preset enablement routes.
- **Agent** — native enablement calls and the palette catalog read from inside a session, agreeing with the CLI.
- **Runtime** — the create-once marker rows across update, restart, uninstall, and reinstall; the delivery outcome per profile for the preset.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
