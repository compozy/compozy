# J-connect-bridge-provider — Connect a bridge provider and receive the first response

An operator starts with an installed provider and finishes with a provider-visible AGH response.
The journey branches by the provider's real setup surface: Slack uses a generated manifest;
WhatsApp, Telegram, and Discord use guided setup; Teams, Google Chat, GitHub, and Linear use the
generic disabled-create and binding flow. GitHub and Linear finish in an issue or Agent Session,
not a fabricated chat conversation.

```mermaid
flowchart TD
    E[Entry: provider extension installed] --> P{Provider family}
    P -->|Slack| S1[Create disabled bridge]
    S1 --> S2[Generate and paste Slack manifest]
    P -->|WhatsApp / Telegram / Discord| W1[Run guided setup or strict JSON]
    W1 --> W2{Validators accept exact provider values?}
    W2 -->|no| W3[Named remediation; no partial secret write]
    W3 --> W1
    W2 -->|yes| W4{Telegram?}
    W4 -->|yes| W5[Daemon registers setWebhook]
    W4 -->|no| W6[Complete Meta or Discord console handoff]
    P -->|Teams / Google Chat| G1[Create disabled bridge and bind conditional slots]
    G1 --> G2[Configure bot endpoint, Chat endpoint, or Pub/Sub]
    P -->|GitHub / Linear| I1[Create disabled repository or organization bridge]
    I1 --> I2[Configure webhook and auth mode]
    S2 --> V[Verify while disabled]
    W5 --> V
    W6 --> V
    G2 --> V
    I2 --> V
    V --> R{Checks actionable and non-failing?}
    R -->|no| F[Repair named slot, mode, or callback]
    F --> V
    R -->|yes| A[Enable and verify public reachability]
    A --> T{Provider surface}
    T -->|chat provider| T1[Send inbound message; inspect route; run real send-test]
    T -->|GitHub / Linear| T2[Create supported comment or Agent Session event; observe reply]
    T1 --> Z[True end: provider shows one real AGH response and structured read-back is ready]
    T2 --> Z
    W3 -.->|abandon after repeated paste failures| X[Close setup before enabling]
    X -.->|resume| XR[Reopen disabled instance; existing bindings remain masked]
    XR --> V
```

```yaml
journey:
id: J-connect-bridge-provider
  name: "Connect a bridge provider and receive the first response"
  value_statement: "I can follow the setup path that actually belongs to my provider and prove the bridge with a visible response."
  personas: [Tessa, Ada]
  entry_points:
    - url: "agh bridge manifest slack; agh bridge setup whatsapp|telegram|discord; agh bridge create"
      origin: direct
    - url: "GET /api/bridges/providers/slack/manifest; bridge create/bind/register/verify/send-test HTTP or UDS routes"
      origin: direct
    - url: "agh.network bridge setup guides"
      origin: external-share
  actions:
    - step: 1
      verb: "Choose the provider's documented setup path"
      expected_observable: "Only real provider-specific commands and conditional credentials are shown"
    - step: 2
      verb: "Create the bridge disabled and bind write-only credentials"
      expected_observable: "The instance and binding metadata are readable while every supplied secret remains masked"
    - step: 3
      verb: "Complete the provider-console callback or manifest handoff"
      expected_observable: "The public URL, local listener, event subscription, and auth mode agree with the provider guide"
    - step: 4
      verb: "Verify, repair, enable, and verify again"
      expected_observable: "Checks name pass, warn, fail, or skipped truthfully and public reachability is attempted only after enablement"
    - step: 5
      verb: "Create an inbound route and perform a real provider response"
      expected_observable: "A chat message, issue comment, review comment, or Agent Activity shows the AGH response exactly once"
  goal:
    observable: "The external provider contains a real AGH response, and bridge detail/routes agree with that provider identity after a fresh read."
    side_effects: [bridge-instance-created-disabled, secrets-bound-write-only, provider-callback-configured, bridge-route-created, provider-message-delivered]
  true_end_state: "After re-reading the bridge and the provider surface, the instance is enabled and healthy or truthfully degraded, and the first response remains visible at the intended target."
  exit:
    natural: "The operator leaves the bridge ready for teammates or issue authors to use."
  abandonment:
    - at_step: 2
      how: "The operator stops after repeated credential-shape failures and closes the setup flow."
      resume: "The disabled instance can be reopened without duplicate creation; existing values stay masked and verification resumes from the next unresolved fact."
  crosses: [extension-catalog, bridge-cli, http, uds, vault, provider-console, provider-webhook, routing, delivery]
```

Automated backbone: `_tests.md` integration 5.7–5.11 and E2E runtime 6.4. Task 10 adds the
persona walk and TTFM measurements that fake-provider contracts cannot supply.
