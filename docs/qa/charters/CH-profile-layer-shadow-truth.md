# CH-profile-layer-shadow-truth: Stack every layer on one key and make the runtime say which one won

```yaml
charter:
  id: CH-profile-layer-shadow-truth
  mission: "As Dora, give one profile its own resources, config, MCP servers, credentials, and memory across all four layers in two workspaces, then interrogate every surface for the same answer to one question — which layer won, and what did it shadow — while proving personal material never lands in the repository and a shadowed write says so instead of claiming success."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-layer-profile-resources
  scenarios: [MS-repo-profile-layer-adoption, MS-layered-config-write-truth, MS-profile-credential-fallback, MS-profile-memory-tier-scope, MS-001, MS-007, MS-014, ET-045]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Plant the same agent, skill, MCP entry, and config key at all four layers — user, personal profile, project base, project per-profile — and read the winner and its shadow list from `agent list`, `skill list`, `config get`, the Settings source badges, and the native config tools. Then plant a repository layer for a profile that does not exist yet and prove it stays dormant with an actionable diagnostic, wakes on create, and sleeps again on rename with the repository files byte-identical throughout."
      - "Write config with no scope, with each explicit scope, and into a shadowed layer: the default target must follow the current context, `--scope user|profile|workspace` must retarget, and a shadowed write must report saved-but-not-applied naming the winning layer. Attempt every denylisted root and `window_manager.global_shortcuts` on a profile layer and require the typed refusal with allowed-prefix guidance and zero file or apply-record residue. Confirm `--scope global` is rejected outright — `user` replaced it."
      - "Set a provider credential inside the non-default profile: prove it lands under the owning profile's vault prefix, is never echoed, refuses a process-environment reference, and that `provider inspect` names profile-override versus user-default while stating plainly that native provider logins stay machine-level. Remove it with owned work present and prove the warning, the acknowledged removal, and the fallback to the user credential — with usage still attributed to the owning profile."
      - "Read and write memory in the profile tier from both profiles with overlapping slugs and search terms: no entry, count, or match highlight may cross, an aggregate memory read must be refused rather than labeled, and workspace-tier entries must stay shared. Then push the retired vocabulary at the two surfaces that speak it directly — `scope-show` must report the `profile` selector, precedence list, and the root under `$COMPOZY_HOME/profiles/<name>/memory/`, and `promote` must move between `profile`, `workspace`, and `agent` selectors and must never land an entry in another profile's tier. No retired value or path may be accepted or emitted anywhere."
    must_avoid:
      - "Extension placement and enablement (CH-profile-extension-declared-install owns those); the palette side of `[cmd_palette]` layering (CH-profile-palette-lens-isolation); default COMPOZY_HOME or ports — use the isolated lab from the bootstrap manifest, and follow the provider-home policy in it for any real credential."
```

## Selection rationale

This is where the ADR-013 hard cut has to hold. The stored scope word moved from `global` to `user`
everywhere at once — config write scopes, settings scope kinds, resource records, vault ref
segments, generated types — and the memory tier moved to `profile` with a durably coupled directory
move. A missed reader does not corrupt data; it rejects a legitimate value or, worse, silently
accepts a retired one. Safety Invariant 9 adds vault owner-prefix containment and the
environment-reference refusal, and Invariant 11 replaces the scalar scope rank with a lattice whose
narrowing takes a meet, not a minimum. The Feature Tour is right here because the risk is a promise
that is simply not kept on one of four layers, on one of five surfaces.

## Evidence and entry points

- **CLI** — `agent list` and `skill list` with LAYER and SHADOWS columns per profile; `config path|get|set|unset` transcripts for every scope arm; the denylist refusals; `secret set|rm` and `provider inspect` output with values redacted; memory list, search, and recall per profile.
- **HTTP and UDS** — `/api/settings` and its sections showing the same effective values and provenance; the vault and provider status reads; the refused aggregate memory read.
- **Web** — Settings source badges and provenance captures for Persona, Hooks, Command palette, and Memory.
- **Agent** — native config and memory projections from inside a session, matching the CLI answer.
- **Filesystem** — before-and-after hashes of every repository file touched by the walk, plus the `config_apply_records` rows naming the layer and path.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
