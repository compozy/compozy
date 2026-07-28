# J-17 — Start a session through the unified runtime selector

The model-selector MVP's headline web path (`model-selector` _spec §3, §6). An operator opens the session-create dialog and, instead of three stacked form fields (provider select · model select · reasoning select) plus a separate refresh/status block, meets **one button-group control** — provider glyph+name · model · reasoning meter — that opens a single popup (search + refresh header, provider rail, grouped model list with availability + capability chips, reasoning footer). The closed trigger starts from the agent's workspace-scoped effective runtime. Leaving it untouched preserves project inheritance; changing it creates one coherent session override. The regression risk lives in that authored-versus-effective boundary, the reset-on-switch rule, durable asynchronous startup, and the provisional custom-ID path.

```mermaid
flowchart TD
    E1[Entry: Agents view → Start session] --> D[Session-create dialog]
    D --> A[Pick agent · AgentCommandSelect]
    A --> RT[Runtime selector trigger shows effective provider·model·reasoning]
    RT -->|click provider segment| RAIL[Popup opens, provider rail focused]
    RT -->|click model segment| SRCH[Popup opens, search focused]
    RT -->|click reasoning segment| RZ[Popup opens, effort strip focused]
    RAIL --> PICKP{Switch provider?}
    PICKP -->|yes| CLR[Emits provider change, model+reasoning reset to default; catalog refetches for new provider]
    PICKP -->|no| BROWSE
    CLR --> BROWSE[Browse shows curated models grouped by provider · Live/Stale/Sign-in]
    SRCH -->|query matches curated/all| BROWSE
    SRCH -->|no match, non-empty query| CUST[Use exact custom model ID row]
    BROWSE --> PICKM[Pick a model]
    PICKM --> RESET{New model advertises current effort?}
    RESET -->|no| DROP[Reasoning resets to provider default '']
    RESET -->|yes| KEEP[Reasoning level kept]
    DROP --> RZBAR[Reasoning footer: levels strip / supported-no-levels note / none note + ACP|catalog badge]
    KEEP --> RZBAR
    RZBAR --> PICKE[Pick effort or leave Default]
    CUST --> COMMITC[Provisional exact ID emitted; validated by adapter at start]
    PICKE --> SUB[Create session]
    COMMITC --> SUB
    SUB --> BODY{Selector changed?}
    BODY -->|no| INHERIT[Omit provider/model/reasoning]
    BODY -->|yes| OVERRIDE[Send coherent provider/model/reasoning override]
    INHERIT --> POST[POST /api/sessions]
    OVERRIDE --> POST
    POST --> ACCEPT[201 durable starting session → navigate immediately]
    ACCEPT --> OK[Active → composer enabled]
    OK --> REOPEN[Reopen dialog for the same agent → trigger again shows effective runtime; per-session override is not persisted]
    REOPEN --> READ[Fresh-read the created session: transcript begins with the chosen model and applied effort — true_end_state]
    ACCEPT -->|custom/unavailable model| FAIL[Durable stopped startup_failure surfaced, no silent default substitution]
    FAIL --> RECOVER[Operator picks a catalog-listed model or signs the provider in → no session against an unusable runtime — recovery terminal]
    D -.->|no providers in workspace| EMPTY[Empty-providers notice; submit disabled]
    RAIL -.->|needs-auth provider| WARN[Rail item dimmed, rows disabled with reason, trigger warning]
```

```yaml
journey:
  id: J-17
  name: "Start a session through the unified runtime selector"
  value_statement: "One fast control shows my effective provider, model, and reasoning truthfully — untouched values keep project inheritance, while explicit choices become one session-scoped override."
  personas: [Bruno, Sol]
  entry_points:
    - url: "web Agents view → Start session (session-create dialog)"
      origin: in-app-nav
    - url: "web agent detail → Start session"
      origin: in-app-nav
  actions:
    - step: 1
      verb: "Open the session-create dialog and read the runtime trigger"
      expected_observable: "One button-group trigger shows the agent-default provider glyph+name, model name, and — only when the model advertises effort — a reasoning meter with 'Default'; no three stacked selects, no separate refresh block."
    - step: 2
      verb: "Click a trigger segment to deep-link into the popup"
      expected_observable: "Provider segment focuses the provider rail; model segment focuses search; reasoning segment focuses the effort strip. ⌘K opens; ↑↓ move; ↵ picks; Esc closes and returns focus to the trigger."
    - step: 3
      verb: "Browse the grouped model list and switch provider from the rail"
      expected_observable: "Browsing shows the curated view grouped by provider with a harness badge and Live/Stale/Sign-in availability; each row carries context/cost/tools/levels chips; switching provider resets model+reasoning to default and refetches that provider's catalog."
    - step: 4
      verb: "Pick a model, then choose a reasoning effort (or leave Default)"
      expected_observable: "Selecting a model whose efforts exclude the current level resets reasoning to provider default; the footer shows the levels strip (with a Default dot) for a model with efforts, a 'provider decides' note for supports-reasoning-without-levels, or a 'no reasoning' note otherwise, plus an ACP|catalog source badge."
    - step: 5
      verb: "Create the session"
      expected_observable: "An untouched selector omits provider, model, and reasoning_effort so project inheritance remains live; an explicit change sends one coherent runtime override. Navigation lands on a durable starting session, whose composer stays disabled until active."
  goal:
    observable: "A session is created with the operator's chosen runtime; the daemon applies the advertised reasoning effort after model selection and before the first prompt; the UI never advertised an effort the runtime could not honor."
    side_effects: [session-created, catalog-fetched-view-all, favorites-recents-localstorage-updated]
  true_end_state: "Reopen the dialog for the same agent: the trigger again reflects workspace-scoped effective runtime (per-session overrides are not persisted to the agent). The created session becomes active with the chosen model and effort, or stops with a durable startup failure and recovery path."
  exit:
    natural: "Operator lands on the live session thread (hands off to J-13/J-11) with the runtime they chose."
  abandonment:
    - at_step: 3
      how: "Operator switches to a needs-auth provider (dimmed rail item) and cannot pick a disabled row."
      resume: "The trigger shows the inline warning; the operator signs the provider in (settings) or picks another provider — no session is created against an unusable provider."
    - at_step: 4
      how: "Operator types an exact custom model ID the catalog doesn't list, then closes the dialog."
      resume: "Reopening, the custom ID is provisional only; on create the accepted session is validated by the active adapter and fails durably with startup_failure rather than silently substituting the provider default."
  crosses: [runtime-selector, model-catalog-view, session-workspace-validation, acp-reasoning-apply, favorites-localstorage, workspace-providers]

design_reference:
  screens:
    - "docs/design/opendesign/provider-model-reasoning-selector.html (normative trigger/popup/reasoning-footer states)"
    - "Storybook systems/runtime/components/RuntimeSelector (trigger default/compact/small/no-reasoning/needs-auth + popup rail/search/reasoning modes)"
  truthful_ui_checks:
    - "Reasoning segment/strip is absent when the selected model advertises no effort (never a control the runtime can't honor) — invariant §7.1."
    - "Meter is hollow + 'Default' when reasoning is unset; filled at the canonical position for the selected level; renders only the model's effort subset."
    - "Needs-auth provider dims its rail item, disables its rows with a reason, and shows the trigger warning; a disabled row never selects."
    - "Untouched effective provider/model/reasoning are omitted from the POST body; only an explicit selector change authors a session override."
    - "A model switch that drops the current effort resets reasoning to provider default (reset-on-switch — invariant §7.6)."

e2e_backbone:
  web:
    - "E2E-web (make test-e2e-web): session-create happy path through the unified selector — pick provider → model → effort → create; the session row shows the choices."
    - "browser-use highest-risk flow: session-create → Claude model → max reasoning, with evidence captured (task 04)."
  runtime:
    - "E2E-runtime (make test-e2e-runtime): session-create-with-effort scenario green; acpmock records model selection before effort, both before the first prompt."
  manual:
    - "Charter CH-028 (Bruno) walks the full trigger→popup→create path incl. reset-on-switch, custom-ID, favorites, keyboard."
  telemetry:
    - "No reasoning-budget environment variable is injected; explicit 'none' issues the RPC, empty default does not (task 04 ACP evidence)."
```
