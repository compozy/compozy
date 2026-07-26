# Administer Network availability and Live policy without enrollment

An administrator changes availability and finite Live defaults/limits, observes a safe disable/re-enable transition, and activates Network-aware extension resources only after explicit confirmation. The true end is a fresh restart where every structured and visual surface agrees, preserved data remains readable, and an ordinary execution is still Local.

```mermaid
flowchart TD
    A[Entry: Settings Network, config.toml, CLI, HTTP, or UDS] --> B[Read availability, epoch, Live defaults/limits, and runtime status]
    B --> C[Apply one valid settings change; config writes stay sequential]
    C --> D{Apply succeeds live or requires restart?}
    D -->|live| E[Fresh read shows new value]
    D -->|restart| F[Restart banner names requirement; daemon restart]
    F --> E
    E --> G[Try a removed key or over-ceiling bound]
    G --> H[Reject deterministically; active config and availability remain unchanged]
    H --> I[Start one bounded Live execution and preserve conversation data]
    I --> J[Disable Network]
    J --> K[New Live admission rejected; active wake canceled/settled; Local work continues; data is read-only]
    K --> L[Re-enable Network; epoch advances; old sources are not re-admitted]
    L --> M[Preview a bundle or extension with a Live requirement]
    M --> N{Current requirement explicitly confirmed?}
    N -->|no| O[Activation/update fails visibly; no partial resource or enrollment]
    N -->|yes| P[Activation persists digest, confirmer, and timestamp; declared channels remain inventory]
    P --> Q[Change requirement digest]
    Q --> R[Prior confirmation clears; reconfirmation is required]
    R --> S[Start an ordinary execution with participation omitted]
    S --> T[Fresh restart and compare Web, CLI, HTTP, UDS, config, activation, and usage reads]
    T --> U[True end: policy and confirmation survive; ordinary execution is Local; preserved Network data is intact]
    F -.->|operator postpones restart| V[Abandon: saved restart-required state remains explicit]
    V -.->|return later| F
```

```yaml
journey:
  id: J-administer-network-live
  name: "Administer Network availability and Live policy without enrollment"
  value_statement: "An administrator can govern whether Live exists, bound its defaults, and confirm extension requirements without any setting or activation silently enrolling work."
  personas: [Bruno, Ada]
  entry_points:
    - url: "web /settings/network and bundle/extension activation surfaces"
      origin: in-app-nav
    - url: "config.toml; agh config/network/bundle verbs; HTTP/UDS settings, status, coordination, and bundle endpoints"
      origin: direct
  actions:
    - step: 1
      verb: "Inspect and update finite Live defaults/limits"
      expected_observable: "Supported values round-trip through Web, config, CLI, HTTP, and UDS; restart-required changes are explicit; removed keys and over-ceiling bounds fail without partial apply"
    - step: 2
      verb: "Disable and re-enable Network around active and Local work"
      expected_observable: "New Live requests fail clearly, in-flight provider work cancels and settles truthfully, Local work continues, preserved data stays readable, and re-enable does not replay old sources"
    - step: 3
      verb: "Preview and activate a Network-aware bundle or extension"
      expected_observable: "A current Live requirement needs explicit confirmation; decline or stale digest fails visibly without partial activation, while declared channels never select participation"
    - step: 4
      verb: "Compare all status and persistence surfaces after restart"
      expected_observable: "Availability epoch, disabled/ready/active state, settings, confirmation evidence, and usage agree; an omitted participation request still resolves Local"
  goal:
    observable: "Administrative policy and extension confirmation are durable, consistent, bounded, and never act as enrollment"
    side_effects: [network-availability-epoch-advanced, active-wake-settled, live-policy-persisted, activation-confirmation-persisted]
  true_end_state: "After a daemon restart, Web, CLI, HTTP, UDS, and config reads agree on availability and finite bounds; preserved conversation data remains intact; stale requirement confirmation is not accepted; and a new ordinary execution is Local with zero Network usage."
  exit:
    natural: "The administrator leaves Network ready or disabled with the state and consequences visible."
  abandonment:
    - at_step: 1
      how: "A restart-required settings change is saved but the administrator postpones restart."
      resume: "The banner remains visible, current runtime state stays truthful, and restarting later applies exactly the saved values once."
  crosses: [settings-web, config-lifecycle, status, availability-store, live-admission, provider-cancellation, usage-ledger, extensions, bundles, hooks, native-tools, cli-http-uds-parity]
```
