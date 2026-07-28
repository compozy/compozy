# J-19 — Choose a default runtime during onboarding

A first-time adopter's first real decision (`model-selector` _spec §6.2). The onboarding default-model step used to hand-roll a provider button grid plus separate model/reasoning selects; the migration deletes the grid and mounts the unified `RuntimeSelector`, keeping the authentication section below. The commit contract is unchanged and load-bearing: picking a provider + model + default reasoning must still fold into a single provider-settings write (`buildOnboardingProviderRequest`) that appends the model to the curated set, sets the models default, and emits `model_curation.default_effort` — then sets the general default provider.

```mermaid
flowchart TD
    E1[Entry: first-run onboarding wizard → Default model step] --> LOAD{Providers loaded?}
    LOAD -->|loading| SPIN[Loading providers…]
    LOAD -->|error| PERR[Provider load error]
    LOAD -->|ok| SEL[Unified RuntimeSelector replaces the old grid]
    SEL --> PICKP[Pick provider from the rail]
    PICKP --> PICKM[Pick a curated model or leave provider default]
    PICKM --> PICKE[Pick a default reasoning effort or leave Default]
    PICKE --> AUTH[Authentication section stays below: native_cli vs bound_secret]
    AUTH -->|native_cli| CONT[Continue]
    AUTH -->|bound_secret| KEY[Env var + optional API key]
    KEY --> CONT
    CONT --> COMMIT{buildOnboardingProviderRequest}
    COMMIT -->|model → curated[] + models.default; reasoning → model_curation.default_effort| PUT[PUT provider settings]
    PUT --> GEN[Set general default provider]
    GEN --> OK[Onboarding advances; default runtime persisted]
    OK --> RERUN[Re-run the step / fresh settings read → the persisted selection shows, sourced from settings: single projection, not a raw config echo]
    RERUN --> LATER[A later session-create inherits the persisted default provider·model·effort — true_end_state]
    SEL -.->|provider change| RESETC[Model/reasoning + entered creds reset for the new provider]
    COMMIT -.->|no model or no reasoning| NOCUR[No model_curation intent written; membership only]
```

```yaml
journey:
  id: J-19
  name: "Choose a default runtime during onboarding"
  value_statement: "On my first run I pick a provider, model, and reasoning depth with the same fast control I'll use everywhere, and it becomes my default — no throwaway grid I never see again."
  personas: [Lea]
  entry_points:
    - url: "web first-run onboarding wizard → Default model step"
      origin: direct
  actions:
    - step: 1
      verb: "Reach the default-model step"
      expected_observable: "The step shows one RuntimeSelector (the hand-rolled provider button grid is gone) with the Authentication section preserved below it; a provider load error or loading state renders in place of the selector."
    - step: 2
      verb: "Pick provider, model, and default reasoning"
      expected_observable: "Provider selection drives model+reasoning; switching provider resets model, reasoning, and any entered credentials; needs-auth providers dim in the rail."
    - step: 3
      verb: "Set authentication (native CLI or bound secret) and continue"
      expected_observable: "The auth section behaves exactly as before; bound_secret requires an env var (or an existing slot) and optionally a pasted key."
    - step: 4
      verb: "Commit the default"
      expected_observable: "One provider-settings write folds the chosen model into the curated set + models.default and emits model_curation.default_effort when both model and reasoning are set; then the general default provider is set. No merged display/cost/effort metadata is written back into config."
  goal:
    observable: "The operator's chosen provider·model·reasoning become the workspace default via the unchanged commit contract; onboarding advances."
    side_effects: [provider-settings-put, general-default-provider-set, curated-membership-appended]
  true_end_state: "The default provider/model/effort persist and drive later session-create defaults; re-running the step shows the persisted selection sourced from settings (single projection, not raw config echo)."
  exit:
    natural: "Onboarding continues to the next step with a real default runtime configured."
  abandonment:
    - at_step: 2
      how: "Operator selects a provider, enters a key, then switches provider."
      resume: "Model, reasoning, and credentials reset for the new provider — no stale key bound to the wrong provider."
    - at_step: 4
      how: "Operator leaves the model at provider default and commits."
      resume: "No model_curation intent is written (membership/default only); the provider default model is used, and no fabricated reasoning is persisted."
  crosses: [runtime-selector, onboarding-provider-request, settings-provider-put, provider-auth-modes, model-catalog-view]

design_reference:
  screens:
    - "docs/design/opendesign/provider-model-reasoning-selector.html (New session · runtime step reused verbatim on onboarding)"
    - "Storybook systems/onboarding StepDefaultModel"
  truthful_ui_checks:
    - "The provider button grid is deleted; one RuntimeSelector drives provider·model·reasoning; the auth section is unchanged."
    - "buildOnboardingProviderRequest inputs are unchanged (model/reasoning/authMode/envVar/apiKey/provider) — the commit body shape is a preserved contract."
    - "Switching provider clears model, reasoning, and entered credentials."
    - "Settings-backed surfaces show the same merged projection as the catalog (no raw curated-config echo — invariant §7.3)."

e2e_backbone:
  web:
    - "E2E-web (make test-e2e-web): onboarding default-model step completes without the grid."
  runtime:
    - "Unit (provider-request.test.ts): commit body folds model + default reasoning as model_curation; native_cli clears credential slots; bound_secret resolves target env."
  manual:
    - "Charter CH-030 (Lea) walks the onboarding default-model pick incl. provider-change credential reset and the auth section."
```
