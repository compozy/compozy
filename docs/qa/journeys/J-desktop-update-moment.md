# J-desktop-update-moment: Take an update at my convenience and never get stranded

A daily user meets the update moment: the app keeps itself current in the background and applies on
consented restart; an app-provisioned runtime updates through the same single experience with
timing consent under in-flight agent work; managed installs get a recommendation, never a write;
every failure leaves the app launchable with a way forward.

```mermaid
flowchart TD
    A[Entry: update-ready indication, compozy update, or Settings Updates] --> B[One daemon check reports runtime and app from the same release]
    B --> C{User consents to restart now?}
    C -->|later| D[Keep working - offer stays, no nagging]
    D --> C
    C -->|now| E[Restart: new app version running, version indicator updated]
    E --> F{Runtime update also available?}
    F -->|app-owned, agent work in flight| G[Timing consent: apply now vs later - never a silent runtime stop]
    G -->|now| H[Quiesce, stop, swap, start, reconnect - both new versions in one update surface]
    G -->|later| D
    F -->|managed install| I[Availability + exact channel command - zero writes, clears after the user updates externally]
    F -->|none| J
    H --> J[True end: About shows channel beta + versions, no residual pending state]
    I --> J
    B -->|apply step fails - locked install dir| K[Failed update reported + manual-download path opens the release page]
    K --> L[App still launchable, install path permissions intact]
    H -->|post-migration boot failure| M[recovery_required sticky, old binary not restarted, resolved by a later signed build]
    C -.->|quit with the update downloaded| X1[Abandon: next launch applies or launches into the new version - the update is never lost]
    B -->|apply through Settings HTTP or UDS| O[Accepted operation id, runtime-first execution, live projection converges]
    O --> J
    B -->|dormant operation blocks progress| P[Cancel through CLI, HTTP, or UDS]
    P --> J
```

```yaml
journey:
  id: J-desktop-update-moment
  name: "Take app and runtime updates at my convenience"
  value_statement: "I stay current without manual downloads, my agent work is never interrupted without consent, and a failed update always leaves me a way forward."
  personas: [Bruno, Dora]
  entry_points:
    - url: "in-app update-ready indication; Settings Updates; compozy update"
      origin: in-app-nav
  actions:
    - step: 1
      verb: "Keep working while a newer app version publishes"
      expected_observable: "Background download completes; a calm ready indication appears — no forced restart"
    - step: 2
      verb: "Consent to restart"
      expected_observable: "The new app version is running and the version indicator reflects it"
    - step: 3
      verb: "Meet a runtime update with agent work in flight (app-owned home)"
      expected_observable: "Timing consent; 'later' keeps everything working; 'now' quiesces, applies, restarts, reconnects"
    - step: 4
      verb: "Meet a runtime update on a managed install"
      expected_observable: "Availability plus the exact channel command; no binary is touched; the surface clears after an external update"
    - step: 5
      verb: "Check the About/update surface"
      expected_observable: "Channel (beta) and versions visible; no stable channel selector exists"
    - step: 6
      verb: "Apply or cancel through a structured settings route"
      expected_observable: "HTTP and UDS return the same accepted, blocked, or canceled operation truth"
  goal:
    observable: "One coherent update experience covering app and app-owned runtime"
    side_effects: [app-binary-replaced-on-restart, runtime-binary-swapped-under-lock, provenance-marker-updated]
  true_end_state: "Both new versions visible in one update surface with no residual pending state; every failure branch (failed apply, malformed feed, post-migration boot failure) leaves the app openable with diagnostics and a manual path."
  exit:
    natural: "The user resumes work on the new versions."
  abandonment:
    - at_step: 2
      how: "Quit with the update downloaded but unapplied."
      resume: "The next launch applies or launches into the new version per platform convention — the update is not lost."
  crosses: [github-release-channel, platform-signing, quiesce-contract, update-lock-journal, install-method-detection, release-pipeline]
```
