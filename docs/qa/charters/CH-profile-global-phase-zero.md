# CH-profile-global-phase-zero: Start with nothing, work across everything, and never land in the home folder

```yaml
charter:
  id: CH-profile-global-phase-zero
  mission: "As Lea, finish first run without pointing at any project folder, work in the Global view, and then try in every way available to produce a home-directory workspace — through the browser, the Add-workspace dialog, the CLI, HTTP, UDS, and a daemon boot on an install that used to have one — proving Global is a view over real folders and that work created there is honestly no-workspace work."
  mode: charter-with-tour
  persona:
    name: Lea
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-scope-global-across-workspaces
  scenarios: [RT-home-workspace-not-registrable, RT-compozy-home-layout, RT-onboarding-skip-to-global, MS-global-scope-no-workspace-work, MS-web-workspace-add-directory-browser, MS-web-workspace-lists-hide-home, MS-web-menubar-global-scope-toggle, RT-web-session-all-workspaces]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Finish onboarding with zero folders and confirm you land on a working desktop rather than a gate, that the chip reads Global, and that nothing anywhere mentions profiles while only `default` exists. Then check every surface that can list a workspace — menu, overview, palette rows, command select, Add workspace — and confirm the home directory is not a row, a pin, or a badge, and that the list is identical in every profile."
      - "Attack registration: `compozy workspace add ~/`, the same call over HTTP and UDS, and equivalent spellings — trailing slash, `$HOME`, a symlink to home, a relative path resolving there. Every one must refuse deterministically with the same reason and create no row; a real project folder registered right after must still succeed."
      - "Boot a daemon on an install that previously carried the home workspace row with work attached: no auto-registration may run, the row must be gone, and every item it held must still read back as no-workspace work with its counts and relationships intact — then restart again and confirm nothing recreated it."
      - "Create work while Global is on and confirm it is no-workspace work owned by the acting profile on the web, CLI, HTTP, and UDS alike. Watch the session catalog with a request log open and prove the daemon narrowed the response rather than the browser hiding rows; drop the stream and reconnect and confirm the boundary survives replay. Finally turn Global off with a remembered project and with none, and confirm the restore and the honest stays-on reason."
    must_avoid:
      - "The profile axis of the same lists and streams (CH-profile-foreign-leak-probe owns it); default COMPOZY_HOME or ports — use the isolated lab from the bootstrap manifest, a genuinely fresh home for the first-run leg, and a production-parity web build."
```

## Selection rationale

Phase 0 is the foundation everything else stands on: it deletes the boot-time registration of the
operator home, makes the home non-registrable, redefines Global as the across-workspaces view whose
creations are no-workspace work, and moves session-catalog filtering into the daemon — the very
enforcement point profile scoping later extends. ADR-007 put those fixes inside this spec rather
than beside it for that reason. If the server-side filter is not actually server-side, every profile
read built on it inherits the flaw silently. Lea is the right persona because the fresh-install,
zero-folder path is hers and because the phase-0 promise is specifically that a newcomer can work
before owning a project folder. The Feature Tour fits: these are advertised behaviors, and the risk
is one of them simply not being kept.

## Evidence and entry points

- **Web** — the first-run completion capture, the desktop with the Global chip, every workspace-listing surface showing no home row, the Add-workspace browser attempting to reach home, and the toggle's stays-on state with its reason.
- **CLI** — the `workspace add ~/` refusal and its path-spelling variants, `workspace list` before and after each attempt, the successful project registration, and the created no-workspace item read back.
- **HTTP and UDS** — the same registration refusals on both listeners with status codes and bodies, and the no-workspace item read on each.
- **Runtime** — a request or proxy log proving the catalog response was already narrowed, the indeterminate-scope response, paired reconnect and replay frames, and pre- versus post-boot counts for every family of work the legacy home row used to hold.
- **Agent** — none required.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
