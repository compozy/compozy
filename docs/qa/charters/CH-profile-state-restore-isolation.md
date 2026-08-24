# CH-profile-state-restore-isolation: Interrupt profile switches and see whose windows come back

```yaml
charter:
  id: CH-profile-state-restore-isolation
  mission: "As Bruno, arrange real work across desktops in two profiles and then interrupt the switching — close mid-arrangement, overlap two switches, archive and unarchive, delete — to prove each profile's desks, active desktop, and attention state come back exactly as left, that nothing crosses in either direction, and that a delete removes precisely what its preview counted."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-restore-per-profile-state
  scenarios: [ET-profile-desktop-restoration, ET-window-manager-layout-recovery, ET-window-manager-multi-client, MS-attention-settings-roundtrip, RT-session-attention-catalog]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Arrange a second desktop with several windows under `default`, switch to a second profile and require a clean single default desktop with none of `default`'s windows, arrange it differently, then switch back and forth confirming both arrangements — including which desktop is active — return exactly as left."
      - "Interrupt the machinery: close the client mid-arrangement and reopen; overlap two profile switches from two clients and confirm registration serialises so the destination is never stranded by a stale unregister; keep a second client on the other profile and confirm neither sees the other's windows and neither is force-switched."
      - "Archive the second profile and confirm the operator lands on `default` with the arrangement retained untouched, then unarchive and confirm it returns unchanged. Delete an empty profile owning saved desks and confirm the preview count, the CLI and DELETE response count, and the actual removal all agree while every other profile's desks are byte-stable."
      - "Mute a workspace in one profile and read the badges in both. Prove the authoritative row lands in `attention_workspace_mutes` for only that profile, no retired `attention.muted_workspaces` config key appears, the other profile still counts that workspace, and deleting the workspace cascades every matching profile row without changing the global delivery booleans."
    must_avoid:
      - "Generic window-manager topology behaviors the existing CH-window-* charters own; default COMPOZY_HOME or ports — use the isolated lab from the bootstrap manifest and a production-parity web build."
```

## Selection rationale

Desktop partitioning is the newest per-profile state and the one built latest, at the composition
root: one window manager per profile behind a registry, each with its own clientstate key, while the
window-manager package itself stayed workspace-keyed. That shape has two known-fragile seams — the
atomic client registration claim during overlapping switches, and the sealed repository during a
delete-time purge — and both only fail under interruption, which is why this is an Interrupt Tour
rather than a feature walk. The attention half attacks the same isolation axis: profile-owned
`attention_workspace_mutes` rows must never behave like a machine-wide setting.

## Evidence and entry points

- **Web** — screenshots of each profile's desks before and after a switch, the fresh-profile clean state, the two-client pair, and the badge counts in both profiles.
- **CLI** — `compozy desktop list` per profile and the delete preview beside the delete result showing the same saved-desktop count.
- **HTTP and UDS** — the window-manager get, preview, and commands routes with an explicit profile, plus `GET/PATCH /api/settings/attention?scope=profile&profile=<name>` on both listeners.
- **Runtime** — the interleaved transcript for overlapping switches and the profile/workspace mute rows before and after workspace deletion.
- **Agent** — none required.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
