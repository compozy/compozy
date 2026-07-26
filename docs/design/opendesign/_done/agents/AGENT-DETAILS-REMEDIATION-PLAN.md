# Agent Details — Root-Cause Analysis and Remediation Plan

## Document status

- **Purpose:** explain why the implemented `agent-details` feature lost substantial visual and interaction quality relative to the approved OpenDesign references, then define the complete remediation required in product code, shared UI, specifications, workflow, tests, and visual evidence.
- **Primary audience:** AGH product engineers, design-system maintainers, spec authors, reviewers, and QA operators.
- **Document type:** explanation plus implementation guide.
- **Normative visual inputs:** `agent-detail.html`, `agent-settings.html`, `agents-list.html`, and `provider-model-reasoning-selector.html`.
- **Repository evidence:** claims about the archived spec and production implementation are taken from the supplied audit conversation. They are intentionally marked as **reported** when they cannot be independently revalidated from this Design Files workspace.
- **Implementation prerequisite:** before changing production code, revalidate every reported path, API, component, and commit against the current AGH worktree. Revalidation may refine filenames; it must not weaken the behavioral or visual contracts defined here.

## Executive decision

The failure was not primarily caused by a bad color palette, incorrect base CSS, or missing low-level UI primitives. The evidence points to a contract and governance failure that propagated into composition code:

1. The product specification and ADR contradicted the approved visual references on two load-bearing decisions: settings topology and page hierarchy.
2. The visual references were not durable inputs to the spec. They were ignored by Git, copied manually, and reduced to prose instead of preserved as immutable evidence.
3. The implementation selected a different route composition and information architecture from the references: settings became a full page, the body-side detail header disappeared, runtime selection was omitted, metrics changed meaning, and multiple hierarchy layers were flattened.
4. Shared primitives reportedly already covered most of the required anatomy, but the feature did not consistently use or compose them according to their semantic contracts.
5. The visual gate accepted implementation-only screenshots. It proved that pages rendered, not that they matched the approved design.
6. Review treated the missing durable reference bundle as follow-up work instead of a blocking contract failure.

The correct response is therefore not a token refresh or a cosmetic pass. It is a structural redesign with a new TechSpec, explicit delete targets, durable reference provenance, route-level modal composition, corrected data semantics, shared component ownership, behavioral tests, and reference-versus-implementation evidence for every material state.

## 1. Evidence model

This document separates evidence into three classes so implementation teams know what is fixed and what must be checked.

### 1.1 Confirmed in the local OpenDesign references

The current files in this workspace establish the following:

- `agent-detail.html` contains a body-side detail header below the route top bar. It includes the agent icon, name, status, invalid-state signal, category path, and a Provider · Model · Reasoning control.
- The detail page exposes Overview, Instructions, Configuration, and Sessions as primary content lanes.
- The Overview metrics are **Active**, **Runtime**, **Failed**, and **Last activity**.
- The reference uses layered but flat warm-dark surfaces, restrained borders, mono operational values, semantic signal colors, compact labels, and narrow spacing increments.
- `agent-settings.html` is a centered modal on a scrim, not a dedicated replacement page. It has a descriptive header, section navigation, scroll-contained form body, persistent footer note, Cancel, and Save changes.
- The settings navigation covers Basics, Runtime, Instructions, Access, MCP servers, and Danger zone.
- The settings Runtime section uses the unified Provider · Model · Reasoning selector.
- The settings modal distinguishes draft changes from immediate state: the modal saves as a unit, while the detail-header runtime control is presented as a live operation.
- `agents-list.html` separates the agent name, structural/category information, provider/model identifiers, origin, status, and actions through distinct type roles rather than treating all metadata as a single uppercase eyebrow.
- `provider-model-reasoning-selector.html` defines one adaptive control with provider, model, and reasoning segments; a shared popup; search; provider rail; availability states; model capability detail; reasoning levels; keyboard behavior; compact and unavailable variants; and context-specific usage.
- The selector contract hides reasoning when the model exposes no selectable effort and surfaces provider-auth or availability problems rather than pretending the choice is valid.

### 1.2 Reported by the repository audit conversation

The supplied audit reports the following production and spec facts. These are sufficiently concrete to plan against, but must be revalidated before editing:

- The archived PRD required a dedicated full-page editor even though the settings reference specifies a modal.
- The archived design spec required no body-side H1 even though the detail reference depends on a body-side `DetailHeader` for hierarchy.
- The detail route replaced the parent page with a child `<Outlet>`, making an overlay route impossible in the implemented composition.
- Status and agent metadata were compressed into the top bar.
- The existing `DetailHeader` primitive was omitted.
- The implementation exposed **Total sessions** and **Resumable** instead of the reference metrics **Active**, **Runtime**, **Failed**, and **Last activity**.
- Runtime duration and failure data reportedly already existed in the fleet signal layer.
- Settings used full-page layout conventions, top-bar actions, card-like form sections, and sidebar navigation styles rather than the modal anatomy.
- The list card placed `category · model · origin` into an `Eyebrow`, incorrectly turning operational metadata into uppercase structural labeling.
- The archived task checked screenshot parity but preserved only two implementation screenshots, with no rendered reference, side-by-side comparison, diff, comparison metadata, or review report.
- Strong Visual Contract Mode was added after the original implementation and therefore did not protect that delivery.
- A review identified the non-durable references but deferred the problem instead of blocking the feature.
- The implementation already received an initial pass and a remediation pass, so AGH's two-touch rule requires the next intervention to be designed as a structural TechSpec rather than a third patch.

### 1.3 Newly confirmed provenance drift

The hashes recorded in the supplied conversation differ from the current files in this workspace:

| Reference | Hash recorded in the conversation | Current local SHA-1 | Consequence |
| --- | --- | --- | --- |
| `agent-detail.html` | `e7caebf120eb75b3f33e358c20e4759d61314238` | `4a4c214402cc83a06ff8ab7c607b9c0d6cfc12bc` | The reference changed after the earlier audit or a different copy was audited. |
| `agent-settings.html` | `e133edbfec2cd8a86c4c096a0ef213ea02d63908` | `290e65b55ce4e5c5c46d357589d4499c4be47f1f` | A bare path is insufficient to identify the approved settings contract. |
| `agents-list.html` | `1d978579dd71c838f7cfbd302d4c0f3428d5e461` | `33fdd10fc9c51c23eb9ae4508a5260b98b4a1f79` | Reviews cannot reproduce prior decisions without a frozen copy. |
| `provider-model-reasoning-selector.html` | `a533382d709468dd429610fed360411e0d029ad8` | `02ed3f076fbc8daae8935fd337aeb67cc591f4a4` | The selector's exact state contract must be versioned with the consuming spec. |

This drift is not a reason to discard the references. It is direct evidence that specs must preserve immutable copies and record hashes. The new TechSpec must choose and freeze the intended version explicitly. No implementation may claim parity against “the current file at this path.”

## 2. What the approved design is actually doing

The references are better than the implementation described in the conversation because they encode a coherent operational hierarchy, not merely nicer colors.

### 2.1 Top bar and detail header have different jobs

The route top bar answers “where am I and what route-level actions are available?” It contains breadcrumb/back navigation, New session, Edit settings, and overflow actions.

The body-side detail header answers “what entity am I operating on, what is its state, and what runtime does it use?” It contains:

- agent identity;
- health and validity signals;
- category or origin context;
- the runtime selector;
- enough breathing room and a divider to establish the page's primary hierarchy.

Compressing these layers into one top bar destroys both. Breadcrumbs, identity, status, runtime, and actions compete in a 48-pixel strip, while the content begins without an anchor. This is an information-architecture defect, not a spacing defect.

### 2.2 The detail page is a cockpit, not a settings form

The Overview is designed for scanning:

- four compact operational metrics;
- a main column for runtime and behavior details;
- a secondary column for metadata and supporting facts;
- controlled surface boundaries and hairline separators;
- visible editing affordances that deep-link to the relevant settings section.

The tabs separate “observe” from “inspect configuration” and “review sessions.” That boundary allows density without turning the entire page into a long form.

### 2.3 Settings preserve context

The settings overlay communicates that the user is editing the agent they were just inspecting. The underlying detail route remains the context, while the modal creates a temporary focused task.

The modal anatomy is part of the contract:

- scrim and overlay shadow establish temporary depth;
- the header explains scope and persistence semantics;
- section navigation keeps a long form navigable without turning every section into a card;
- the form body owns scrolling;
- the footer remains visible and explains that changes affect new sessions;
- destructive actions are isolated in Danger zone and require nested confirmation;
- close, Cancel, Escape, and browser navigation all have explicit dirty-state behavior.

A full-page replacement loses the entity context, changes back-navigation semantics, weakens the save boundary, and makes the UI feel like a generic administration form.

### 2.4 The runtime selector is a capability surface

The Provider · Model · Reasoning selector is not three adjacent dropdowns and not decorative metadata. It reconciles multiple runtime facts:

- available providers and their authentication state;
- provider-specific model catalogs;
- model capabilities and availability;
- per-model reasoning levels;
- provider defaults;
- runtime-advertised options that may be more current than static catalog data;
- compact and full contexts;
- keyboard navigation and search.

Omitting it from agent detail is both a visual regression and an agent-management regression. Replacing it with a static label would preserve appearance while losing the feature's operational purpose.

### 2.5 Typography communicates data type

The references use type roles deliberately:

- sans-serif, stronger weight, and negative tracking for entity names and section titles;
- mono for identifiers, model names, counts, durations, prices, context windows, and session IDs;
- uppercase labels only for structural eyebrows and compact metric labels;
- muted body text for descriptions and persistence notes;
- semantic color only for action, success, warning, danger, and availability.

Putting category, model, and origin into one `Eyebrow` erases the distinction between taxonomy, runtime identity, and provenance. The fix is semantic markup and component composition, not a smaller font size.

## 3. Root-cause analysis

### 3.1 Primary cause: the written spec overruled the approved reference

The reported PRD and design spec did not merely omit details. They made decisions opposite to the references:

- **Reference:** settings is a modal over detail. **Reported spec:** settings is a dedicated full page.
- **Reference:** entity identity lives in a body-side detail header. **Reported spec:** no body-side H1.

Once these decisions were frozen in an ADR and task breakdown, a faithful implementer could follow the written spec and still produce the wrong product. This is the strongest root cause because it upstreamed the drift into architecture.

Required correction:

- supersede the conflicting ADR clauses;
- state that named visual references are normative for topology, hierarchy, anatomy, density, and behavior unless a runtime-truth delta is explicitly authorized;
- require the spec to reconcile contradictions before approval rather than leaving them to implementation review.

### 3.2 Primary cause: reference provenance was not durable

An ignored local HTML file is not a reliable product contract. It can change, disappear, or be unavailable in isolated worktrees. The hash drift in Section 1.3 proves this is an active risk.

Required correction:

- copy approved references into the spec's `_refs/` directory;
- record original path, copied path, cryptographic hash, approval timestamp, viewport, initial state, and normative scope;
- render the frozen copy into deterministic images;
- never substitute a prose summary for the original reference;
- fail preflight when a named reference cannot be resolved or hashed.

### 3.3 Primary cause: visual verification checked existence, not parity

An implementation screenshot can prove that the page renders. It cannot prove that the page matches a reference. The archived evidence reportedly had no reference rendering, no paired state, no diff, and no written divergence review.

Required correction:

- define a state matrix before implementation;
- capture `reference.png` and `implementation.png` at identical viewport and state;
- produce side-by-side and diff artifacts;
- record structural divergences separately from pixel noise;
- require zero unresolved blocking structural mismatches;
- make the evidence bundle part of task completion, not an optional reviewer aid.

### 3.4 Secondary cause: route composition made the correct design impossible

If the child route replaces the parent outlet, `/agents/$name/settings` cannot render as an overlay on `/agents/$name`. The route architecture forced the UI into a full-page replacement.

Required correction:

- keep the detail shell mounted for the settings child route;
- render the child route into an overlay slot or route-aware modal host;
- preserve a deep-linkable URL and correct browser history;
- support direct navigation to the settings URL by loading the detail context behind the modal;
- define close behavior as navigation back to detail, not an arbitrary redirect.

### 3.5 Secondary cause: shared components were treated as inventory, not contracts

The audit reports that `DetailHeader`, `Dialog`, `MetadataList`, `Metric`, `LaneTabs`, `Pill`, `RadioCard`, and `Button` already existed. Availability alone is insufficient. A feature can import shared primitives and still compose the wrong hierarchy.

Required correction:

- map every reference anatomy region to an owner before coding;
- use generic primitives from `@agh/ui`;
- keep agent-specific composites in the agent system;
- add missing variants to the owning primitive only when the need is generic;
- add Storybook states that demonstrate the semantic contract, not just default rendering;
- prohibit local shadow copies and page-specific reimplementations.

### 3.6 Secondary cause: data truth and visual contract diverged without reconciliation

The implemented metrics reportedly changed from Active, Runtime, Failed, and Last activity to Total sessions and Resumable even though the required data already existed elsewhere. That is not a harmless product choice: it changes what the cockpit communicates.

Required correction:

- define each metric semantically and identify its source of truth;
- prove workspace scope and aggregation rules;
- render an honest unavailable state when the runtime cannot provide a value;
- never replace a missing value with a more convenient metric without an approved contract change.

### 3.7 Contributing cause: review governance allowed known debt to ship

The review reportedly noticed that references were ignored and non-durable but accepted follow-up remediation. For a named visual contract, missing evidence invalidates the parity claim and must be blocking.

Required correction:

- classify missing or stale visual-contract evidence as a P0 review failure;
- prevent “follow-up” disposition for contract provenance, reference/implementation pairing, and unresolved structural mismatches;
- require reviewers to cite the evidence bundle, not personal visual judgment.

## 4. Issue register

Every issue below must be closed by a specific workstream and acceptance artifact. “Looks better” is not a valid resolution.

| ID | Severity | Issue | Required resolution |
| --- | --- | --- | --- |
| AD-001 | P0 | PRD/ADR requires full-page settings contrary to the approved modal. | Supersede the conflicting decision in a new TechSpec and list the full-page settings composition as a delete target. |
| AD-002 | P0 | Design spec removes the body-side detail header contrary to the reference. | Restore the content `DetailHeader` and restrict the top bar to navigation and route actions. |
| AD-003 | P0 | References are mutable, ignored, or unavailable in isolated worktrees. | Freeze approved copies under the new spec's `_refs/` directory with hashes and provenance. |
| AD-004 | P0 | Earlier and current reference hashes differ. | Select the authoritative version explicitly; record both superseded and selected hashes. |
| AD-005 | P0 | Screenshot evidence contains implementation-only images. | Produce paired reference and implementation captures for every required state. |
| AD-006 | P0 | No structural comparison or divergence review exists. | Add side-by-side, diff, machine metadata, and a human `review.md` with zero blocking mismatches. |
| AD-007 | P0 | Child routing replaces the detail page, preventing an overlay. | Refactor the route shell so settings renders through a modal/overlay outlet over a mounted detail context. |
| AD-008 | P0 | Settings is implemented as a generic full-page form. | Implement the centered, scroll-contained, deep-linkable settings modal anatomy. |
| AD-009 | P1 | Settings selection reportedly uses a sidebar active rail inconsistent with the flat selected model. | Use the shared selected-row treatment without a decorative accent rail. |
| AD-010 | P1 | Settings loses the descriptive header and persistence footer. | Restore header context, dirty indicator, footer note, Cancel, and Save boundary. |
| AD-011 | P0 | Agent identity and status are compressed into the top bar. | Restore body identity, health, validity, category/origin, and runtime placement. |
| AD-012 | P0 | Unified runtime selector is omitted from agent detail. | Integrate the shared selector with real provider/model/reasoning data and mutation semantics. |
| AD-013 | P0 | Runtime mutation concurrency and failure behavior are unspecified. | Define CAS/version behavior, pending state, rollback, conflict refresh, error message, and cache invalidation. |
| AD-014 | P0 | Overview metrics do not match the approved contract. | Implement Active, Runtime, Failed, and Last activity from canonical workspace-scoped data. |
| AD-015 | P1 | Panels, dividers, density, and hierarchy are flattened. | Recompose with canonical surfaces, hairlines, token spacing, and clear main/aside grouping. |
| AD-016 | P1 | Agent-card metadata is collapsed into an uppercase eyebrow. | Separate category, runtime identity, provenance, status, and actions using their correct type roles. |
| AD-017 | P1 | Local spacing and type tuples drift from shared semantics. | Use exported tokens and primitives; remove local copies where a canonical owner exists. |
| AD-018 | P0 | UI may imply unsupported controls or values. | Reconcile every reference element with daemon truth; remove or label unsupported states and record authorized deltas. |
| AD-019 | P1 | Primitive reuse was not mapped before implementation. | Add a component ownership matrix to the TechSpec and enforce the `@agh/ui` reuse gate. |
| AD-020 | P1 | Shared primitive gaps, if any, are unproven. | Audit inventory first; add only generic gaps to `packages/ui` with story and behavior tests. |
| AD-021 | P0 | Visual state coverage is undefined. | Freeze a deterministic state matrix for valid, invalid, loading, empty, error, dirty, conflict, responsive, and destructive flows. |
| AD-022 | P0 | Behavioral tests can pass while hierarchy is wrong. | Add route, mutation, focus, dirty-state, metric-semantic, and workspace-isolation tests at their owning layers. |
| AD-023 | P1 | Modal accessibility behavior is unspecified. | Implement accessible naming, focus trap, focus return, Escape policy, inert background, and keyboard section navigation. |
| AD-024 | P1 | Responsive modal and detail behavior are not acceptance criteria. | Define compact detail, tab overflow, metric reflow, modal navigation reflow, and no-horizontal-scroll checks. |
| AD-025 | P0 | Review can defer missing visual evidence. | Make missing provenance and parity bundles blocking in spec, task, review, and final verification workflows. |
| AD-026 | P1 | QA tracking may still mark changed visual behavior as tested. | Reset affected agent scenarios to `untested`; add new content-addressed scenarios for new runtime-edit behavior. |
| AD-027 | P1 | Public behavior changes may not propagate to docs and the official AGH skill. | Audit and update public routes, API/CLI/UDS guidance, and `skills/agh/` when contracts change. |
| AD-028 | P0 | Workspace scope of metrics and runtime mutation is not proven. | Propagate and test `workspace_id` through route, query, cache, mutation, events, and store boundaries. |
| AD-029 | P1 | List, detail, and settings may drift independently again. | Define shared agent presentation contracts and capture all three surfaces in one visual-contract suite. |
| AD-030 | P1 | The old implementation can survive beside the redesign. | Execute explicit delete targets; do not keep compatibility layouts, aliases, or duplicate selectors. |

## 5. Target product architecture

### 5.1 Route topology

The target route model is:

```text
/agents
└── /agents/:name                 agent detail shell and default lane
    ├── ?tab=overview             observation cockpit
    ├── ?tab=instructions         instruction files
    ├── ?tab=configuration        read-oriented configuration
    ├── ?tab=sessions             session catalog
    └── /agents/:name/settings    settings modal over the mounted detail shell
```

Required navigation behavior:

1. Opening settings from detail pushes `/agents/:name/settings`.
2. The detail page remains mounted and visually present behind the scrim.
3. Closing a clean modal navigates to `/agents/:name` while preserving the previous detail tab where practical.
4. Direct navigation to `/agents/:name/settings` loads the detail shell and opens the modal after the agent context resolves.
5. Browser Back closes the modal before leaving the detail route.
6. A missing agent renders the canonical not-found state; it must not show an empty modal over a broken detail page.
7. Permission or workspace mismatch uses the canonical access error and cannot leak whether an agent exists in another workspace.

Do not implement this with a second, visually similar full-page settings route kept “for compatibility.” AGH is greenfield alpha; the old topology is a delete target.

### 5.2 Detail page anatomy

The target order is:

1. route top bar;
2. optional diagnostics banner;
3. body-side `DetailHeader`;
4. detail lane tabs;
5. active lane content.

The `DetailHeader` must expose:

- agent icon or canonical fallback;
- exact agent name;
- enabled/disabled or active status using semantic signal treatment;
- invalid-definition state when diagnostics exist;
- category path and origin when supplied by the runtime;
- the shared runtime selector in a stable trailing region;
- responsive behavior that moves the runtime selector below identity before truncating critical information.

The top bar must not duplicate the entire header. It owns breadcrumbs and actions only.

### 5.3 Overview lane

The Overview must render the following metric contract:

| Metric | Definition | Empty/unavailable behavior |
| --- | --- | --- |
| Active | Count of sessions for this agent currently in an active runtime state within the selected workspace. | `0`, never an em dash when the query succeeded. |
| Runtime | Sum of elapsed runtime across the agent's sessions according to the canonical duration semantics. | `—` only when duration is genuinely unavailable; tooltip or accessible description explains why. |
| Failed | Count of sessions whose terminal state is failure within the selected scope. | `0` after successful query. |
| Last activity | Relative time from the latest scoped agent/session event accepted as activity by the backend contract. | `Never` when no activity exists; `—` only on data unavailability. |

The metric query must not mix global, other-workspace, or stale cached data. The TechSpec must identify whether Runtime is all-time, retained-window, or filtered-window; the label and description must match that backend truth.

Below metrics, the page should use the reference's main/aside hierarchy:

- runtime summary and edit affordance;
- permissions/access summary;
- instruction summary or canonical preview;
- metadata and origin details;
- definition diagnostics;
- other facts already supported by the runtime.

Do not invent metrics, provider capabilities, or counts to fill visual slots.

### 5.4 Instructions and Configuration lanes

The detail page remains read-oriented:

- Instructions can switch among canonical instruction files only when those files exist in the product contract.
- Configuration groups runtime, permissions, tools, toolsets, and MCP information by meaning.
- “Edit” affordances open settings at the corresponding section through the settings route, not a separate local drawer or duplicate form.
- Code and identifiers use the mono role; explanatory prose does not.
- Empty sections use honest, task-oriented empty states rather than hidden cards or placeholder content.

### 5.5 Sessions lane

The Sessions lane must preserve:

- accurate count and live indicators;
- state filters with URL- or router-owned state when shareability is part of the existing product convention;
- status semantics from the canonical session state machine;
- stable tabular numerics and identifiers;
- keyboard-accessible row actions;
- explicit loading, empty, partial-error, and pagination states.

The Overview metric contract and Sessions lane must derive from the same backend semantics. A user must not see “2 active” in one region and a different active count in another because the queries or filters disagree.

### 5.6 Settings modal anatomy

The settings route must use the shared dialog foundation and the reference anatomy:

- centered modal with bounded width and height;
- dark scrim and overlay-only shadow;
- header with icon, `Agents · Settings` eyebrow, `Edit <agent>`, persistence explanation, dirty pill, and close button;
- left section navigation at desktop sizes;
- compact horizontal/wrapping navigation below the responsive threshold;
- a single scroll owner for the form body;
- persistent footer with scope note, Cancel, and Save changes;
- nested confirmation dialog for deletion;
- no card around every field group;
- flat selected-section treatment;
- no sidebar-style accent rail.

Required sections:

1. **Basics:** immutable name, category path, and only other canonical identity fields.
2. **Runtime:** unified Provider · Model · Reasoning selector, command/default behavior, and permissions where the production domain owner places them.
3. **Instructions:** canonical prompt/instruction inputs with real validation.
4. **Access:** allow/deny tools, toolsets, and capability constraints supported by the runtime.
5. **MCP servers:** truthful configured resources, their scope, and supported editing behavior.
6. **Danger zone:** delete action, retention consequence, and typed or explicit confirmation according to the destructive-action convention.

Dirty-state contract:

- fields edit a modal draft;
- Save validates and submits one coherent mutation or an explicitly documented transaction sequence;
- Cancel and clean close discard no persisted state;
- closing dirty settings asks for confirmation;
- a failed save keeps the draft, exposes the field or form error, and allows retry;
- successful save updates the detail cache and closes or remains open according to the established AGH form convention;
- no production error is discarded.

### 5.7 Live runtime mutation

The detail-header runtime selector is distinct from the modal draft. It is an immediate operation and needs a production-grade mutation contract.

Required sequence:

1. Resolve current provider, model, reasoning effort, and agent version from workspace-scoped data.
2. Open the shared selector with runtime/catalog options.
3. Disable impossible combinations and explain unavailable providers.
4. On selection, submit a version-aware update.
5. Show a pending state without collapsing the control.
6. On success, update the canonical agent query, dependent detail summaries, and settings initial state.
7. On conflict, refresh server truth, explain that the agent changed elsewhere, and require the user to choose again when necessary.
8. On transport or validation failure, restore the server-confirmed value and show a specific error.
9. Never leave the closed trigger displaying an optimistic value that the server rejected.

The exact concurrency primitive may be an ETag, revision, compare-and-swap token, or existing AGH version field. The implementation must reuse the current contract rather than invent a web-only versioning scheme.

### 5.8 Agent list presentation

List rows and cards must share the same information grammar:

- name is the primary line;
- category is structural context, not a concatenated runtime string;
- provider/model is operational identity and uses mono only where appropriate;
- origin is subdued provenance;
- status is a semantic pill or dot, not decorative color;
- actions remain separate from metadata;
- row and card views expose the same important facts even if their composition differs.

The shared `Eyebrow` must be used only for genuine structural labels. Do not solve the current drift with a one-off class that looks like normal text while retaining eyebrow semantics.

## 6. Design-system and shared-UI remediation

### 6.1 What must not change by default

Do not change the global palette, surface ramp, type scale, radius system, or base CSS merely to make the current feature look closer to the references. The conversation's audit concluded that the canonical design system already describes the required dark, flat, dense posture, and the local references demonstrate that this posture is achievable with the existing token family.

A token change is justified only if all of the following are true:

1. the reference requires a semantic value that no token expresses;
2. at least two production consumers need the value;
3. the value belongs to the shared design language rather than the agent domain;
4. the change is made in `packages/ui/src/tokens.css`;
5. generated `DESIGN.md` regions are refreshed through codegen;
6. stories and affected surfaces are visually verified.

### 6.2 Component ownership matrix

The TechSpec must validate and then use this ownership model:

| Need | Preferred owner | Expected implementation posture |
| --- | --- | --- |
| Route top bar | existing web shell | Breadcrumbs and route actions only. |
| Entity detail header | `@agh/ui` `DetailHeader` plus agent-domain slots | Reuse shared anatomy; agent system supplies status, metadata, and runtime action. |
| Lane tabs | `@agh/ui` `LaneTabs` | URL/router state and accessible tab semantics. |
| Metrics | `@agh/ui` `Metric` in an agent-domain grid | Shared typography and value semantics; domain controls the four metrics. |
| Metadata rows | `@agh/ui` `MetadataList` | No local key/value visual clone. |
| Status and validity | `@agh/ui` `Pill` | Semantic tones only. |
| Settings overlay | `@agh/ui` dialog primitives plus agent settings composite | Shared focus and overlay behavior; domain owns sections and persistence. |
| Permissions | existing `RadioCard`/form primitives | No ad hoc selectable cards. |
| Provider/model/reasoning | shared runtime selector | One component and data contract across agent create, settings, sessions, onboarding, and detail where supported. |
| Buttons and icon actions | `@agh/ui` `Button` | Canonical sizes, focus, disabled, pending, and destructive variants. |
| Agent cards/rows | agent-domain composites | Shared list semantics; do not promote domain layout to generic UI prematurely. |

Before adding any new component, inspect `packages/ui/src/index.ts` and the production recipes. If a generic primitive is missing, add it to `packages/ui` with a story and behavior test. If the need is specific to agents, add a domain-prefixed composite under the agent system.

### 6.3 Stories as design contracts

Add or update stories for meaningful shared states, not screenshots of implementation details:

- `DetailHeader`: default, warning/invalid, long name, action overflow, compact width.
- Runtime selector: default, compact, no reasoning, provider unavailable, loading catalog, empty results, mutation pending, and disabled.
- Settings dialog composition: long form with section navigation, dirty state, validation error, conflict message, and compact viewport.
- Metric: normal value, zero, unavailable, success/danger semantic value, long relative time.

Stories do not replace reference parity. They protect shared primitives; the agent feature still needs end-to-end captures.

### 6.4 DESIGN.md enforcement

The design system is not the main failure point, so avoid duplicating the entire remediation into `DESIGN.md`. Only add or refine lean, reusable rules if the current document lacks them:

- named visual references are normative when a feature explicitly cites them;
- route top bars and content detail headers have distinct responsibilities;
- overlays use overlay depth while content remains flat;
- `Eyebrow` is reserved for structural labels;
- implementation-only captures never prove visual parity.

Workflow-specific provenance and evidence mechanics belong in spec/task/verification skills, not in the permanent design-system prompt.

## 7. Specification and workflow hardening

### 7.1 Create a new structural TechSpec

The redesign must begin with a new TechSpec because the existing behavior has already received two implementation touches. The TechSpec must:

- supersede the full-page-settings portion of the prior ADR;
- supersede the topbar-only identity decision;
- freeze the selected visual references;
- define route topology and direct-link behavior;
- define the metric data contract;
- define live runtime update semantics;
- define settings draft/save semantics;
- define workspace scope;
- include the component ownership matrix;
- include the visual state matrix;
- include every delete target;
- include Web/Docs Impact;
- include extensibility, agent-manageability, and config-lifecycle analysis;
- include the AGH Impact Audit;
- co-ship a test contract that assigns each invariant to an owning layer and canonical suite.

### 7.2 Required reference manifest

The new spec should contain a small machine-readable manifest, for example:

```yaml
references:
  - id: agent-detail-overview
    source_path: docs/design/opendesign/agents/agent-detail.html
    frozen_path: _refs/agent-detail.html
    sha256: <computed-from-frozen-copy>
    viewport: { width: 1440, height: 1000, device_scale_factor: 1 }
    initial_state:
      route: /agents/release-captain?tab=overview
      agent_state: invalid-active
      runtime: codex/gpt-5.4/medium
    normative_for:
      - information-architecture
      - detail-header-anatomy
      - density
      - metric-layout
      - settings-entry
    authorized_deltas: []
```

The concrete schema may follow existing AGH conventions, but it must answer these questions unambiguously:

- Which bytes were approved?
- Which state was rendered?
- Which aspects are normative?
- Which runtime-truth differences are authorized?
- Which implementation states must be paired against it?

### 7.3 Spec preflight changes

The spec workflow should fail early when:

- a prompt names a visual file that cannot be read;
- the file is ignored or outside the durable task directory and no frozen copy is created;
- a PRD or ADR changes modal-versus-page topology without reconciling the reference;
- a spec changes primary hierarchy, navigation, or visible metrics without documenting the delta;
- the design reference implies unsupported runtime behavior and the conflict is not resolved;
- the reference version has changed since approval.

The preflight should not force every design detail into prose. Its purpose is to preserve the original source and force decisions about contradictions.

### 7.4 Task-generation changes

Every implementation task that consumes a visual contract must include:

- reference IDs and frozen paths;
- target states owned by the task;
- component ownership and reuse requirements;
- runtime-truth deltas;
- delete targets;
- tests assigned from the TechSpec test contract;
- visual evidence outputs;
- P0 completion criteria.

Tasks must not say only “match the mock.” They must identify the anatomy and behavior under implementation while retaining the frozen mock as the source of visual truth.

### 7.5 Review and final-verification changes

For named visual contracts, SHIP status requires:

- the frozen reference manifest validates;
- each required state has a reference and implementation capture;
- capture metadata matches viewport, theme, data fixture, and initial state;
- side-by-side and diff files exist;
- `review.md` classifies every visible delta;
- no structural delta remains blocking;
- runtime-authorized deltas are linked to the TechSpec decision;
- keyboard and responsive behavior pass their behavioral gates;
- the final monorepo verification passes after scoped iteration gates;
- any QA lab is torn down with clean evidence.

Missing evidence is not “non-blocking follow-up.” It means parity has not been established.

## 8. Implementation workstreams

### Workstream 0 — Revalidation and TechSpec

**Goal:** turn the reported diagnosis into a current, approved implementation contract.

Actions:

1. Re-open the archived feature spec and current production routes.
2. Confirm or correct every reported filename and component name.
3. Select the authoritative versions of the four references.
4. Freeze them under the new TechSpec with hashes and rendered baselines.
5. Document every conflict between prior prose and references.
6. Supersede conflicting ADR decisions.
7. Approve the target route, data, and component architecture.

Exit criteria:

- approved TechSpec and test contract;
- immutable references;
- explicit delete targets;
- no unresolved product-vs-reference contradiction.

### Workstream 1 — Spec workflow provenance enforcement

**Goal:** prevent future features from losing their normative visual inputs before implementation starts.

Actions:

1. Add reference discovery and frozen-copy behavior to the owning spec workflow.
2. Add manifest validation.
3. Add contradiction checks for topology, hierarchy, and supported behavior.
4. Ensure task generation carries reference IDs and state ownership forward.
5. Keep the instruction change lean; do not bloat `AGENTS.md` or duplicate Visual Contract Mode prose.

Exit criteria:

- a test fixture with an ignored external reference produces a frozen copy;
- changed reference bytes fail validation;
- a missing reference blocks spec approval;
- generated tasks retain reference and state identifiers.

### Workstream 2 — Shared primitive and runtime-selector audit

**Goal:** verify that production uses canonical primitives and close only real shared gaps.

Actions:

1. Inventory exported UI primitives.
2. Map the target anatomy to existing components.
3. Consolidate divergent provider/model/reasoning pickers behind one shared selector contract where not already complete.
4. Add missing generic variants to `packages/ui`, not the agent page.
5. Add stories and behavior tests for shared states.

Exit criteria:

- no local generic primitive shadows `@agh/ui`;
- selector consumers share option derivation and state semantics;
- shared additions have stories and tests;
- production source files remain below the architecture cap.

### Workstream 3 — Agent detail route and cockpit

**Goal:** restore the detail hierarchy and correct operational data.

Actions:

1. Refactor the route shell to remain mounted for settings.
2. Restore the body-side `DetailHeader`.
3. Place status, validity, category/origin, and runtime control in their reference roles.
4. Restore Overview, Instructions, Configuration, and Sessions lanes.
5. Implement the four canonical metrics from scoped data.
6. Recompose panels, metadata, and actions with shared primitives.
7. Add loading, empty, invalid, partial-error, long-content, and responsive states.

Exit criteria:

- direct and nested routes behave correctly;
- metrics match backend semantics;
- hierarchy matches the frozen reference;
- no unsupported runtime claims appear;
- paired visual states pass.

### Workstream 4 — Settings overlay redesign

**Goal:** replace the full-page settings implementation with the approved modal interaction.

Actions:

1. Render settings through the route-aware dialog host.
2. Build the six reference sections from canonical form primitives.
3. Implement draft, dirty, validation, save, cancel, and conflict behavior.
4. Add nested delete confirmation.
5. Implement focus management, Escape behavior, focus return, and compact reflow.
6. Delete the full-page layout, top-bar actions, sidebar rail treatment, and duplicate local form chrome.

Exit criteria:

- deep linking, Back, close, and dirty confirmation are deterministic;
- save failures preserve drafts;
- the modal has one scroll owner and a persistent footer;
- accessibility checks pass;
- paired visual states pass.

### Workstream 5 — Live runtime mutation

**Goal:** make the header selector real, truthful, and concurrency-safe.

Actions:

1. Reuse provider and model catalog queries.
2. Derive reasoning options from runtime-advertised configuration first and canonical fallback second, according to the current runtime contract.
3. Wire version-aware update semantics.
4. Implement pending, conflict, auth-required, invalid-combination, and failure states.
5. Synchronize caches and the settings initial draft after success.
6. Expose the same operation through agent-manageable surfaces when public behavior changes.

Exit criteria:

- no stale optimistic value survives rejection;
- workspace and agent identity are included in every boundary;
- conflicts are recoverable;
- selector behavior is consistent across supported consumers.

### Workstream 6 — Agent list typography and information hierarchy

**Goal:** align row and card composition with the reference without cloning the prototype CSS.

Actions:

1. Separate structural category, runtime metadata, provenance, status, and actions.
2. Remove incorrect eyebrow composition.
3. Reuse canonical mono, muted, pill, and action primitives.
4. Verify long names, missing category, unavailable provider, invalid agent, and compact widths.

Exit criteria:

- card and row views communicate the same facts;
- uppercase text appears only in valid structural roles;
- focus, hover, and action behavior are accessible;
- paired list captures pass.

### Workstream 7 — Visual evidence, QA tracking, docs, and final gate

**Goal:** prove the redesign and prevent false-positive completion.

Actions:

1. Capture the complete visual state matrix.
2. Generate side-by-side and diff artifacts.
3. Resolve every structural mismatch.
4. Reset or add QA scenarios as `untested`.
5. Update public docs and the official AGH skill when behavior changes.
6. Run scoped frontend gates during iteration.
7. Run `make verify` once at completion.
8. Run the implementation peer-review loop until SHIP.
9. Tear down every QA process and cite clean teardown evidence.

Exit criteria:

- validated visual-contract bundle;
- QA tracker impact committed;
- docs/skill impact resolved;
- full verification passes;
- external implementation review reports SHIP;
- no long-lived process remains.

## 9. Delete targets

The TechSpec must name concrete files after revalidation, but the semantic delete list is already clear:

- the full-page settings route composition;
- any route behavior that replaces detail with settings instead of overlaying it;
- settings-specific use of the page top-bar action slot;
- sidebar-derived active rail styling in settings;
- local settings header/footer chrome duplicated from shared dialog anatomy;
- topbar-only agent identity and status composition;
- the substituted Total sessions and Resumable metrics in the Overview cockpit;
- local generic metric, detail-header, metadata-list, button, pill, radio-card, or dialog clones;
- divergent provider/model/reasoning selector variants that the shared selector supersedes;
- obsolete tests that assert the deleted topology or duplicate stronger invariants;
- stale screenshots and parity claims that lack reference pairings;
- aliases, compatibility flags, and dual layout paths introduced solely to preserve the old alpha behavior.

Delete targets must be removed in the same change. Do not leave dead routes or CSS “in case the old page is needed.”

## 10. Test contract

Before editing or adding tests, confirm the invariant, owning layer, and canonical suite. The table below is the intended placement model; current suite names must be revalidated.

| Invariant | Owning layer | Canonical suite | What the test must prove |
| --- | --- | --- | --- |
| Settings child route preserves detail and opens an overlay. | Router/integration | Existing agent route integration suite | Direct link, in-app navigation, Back, close, and missing-agent behavior. |
| Modal focus is contained and returned. | Shared dialog or feature integration, depending on ownership | Existing dialog accessibility suite plus one feature composition test | Initial focus, Tab cycle, Escape, inert background, and return to Edit settings. |
| Dirty settings cannot be discarded silently. | Agent settings feature | Existing settings form suite | Change field, attempt close/Back/Cancel, confirm keep editing or discard. |
| Save preserves draft on failure. | Agent settings mutation | Existing settings mutation suite | Server error remains visible; values are not reset; retry succeeds. |
| Live runtime update is version-aware. | Query/mutation integration | Existing agent mutation suite | Correct revision is sent; conflict refreshes truth; rejected optimistic state rolls back. |
| Reasoning options follow selected model capability. | Shared selector | Selector behavior suite | Segment hides when no efforts exist; invalid prior effort resets according to contract. |
| Provider unavailable state is truthful. | Shared selector/query integration | Selector behavior suite | Auth-required provider cannot be committed and exposes a useful action/diagnostic. |
| Overview metrics use canonical semantics. | Agent data adapter/query | Existing agent detail data suite | Active, Runtime, Failed, and Last activity map from backend fields correctly. |
| Metrics are workspace isolated. | Backend/API/store plus web query boundary | Existing workspace isolation suites | Another workspace's sessions cannot affect count, duration, failure, or activity. |
| Overview and Sessions counts agree. | Feature integration | Existing agent detail suite | Same fixture yields consistent active and total facts across lanes. |
| Invalid agents show diagnostics without hiding identity. | Feature integration | Existing agent detail suite | Header remains present; invalid pill/banner and actionable diagnostic render. |
| List card metadata retains semantic roles. | Agent list component | Existing agent list suite | Accessible names and visible fields are correct; do not assert CSS literals. |
| Deletion explains retention and requires confirmation. | Agent settings feature | Existing destructive-action suite | Cancel is safe; confirmed delete uses canonical mutation and navigates predictably. |
| Responsive layouts remain operable. | Browser E2E/visual contract | Existing web E2E plus screenshot bundle | No horizontal scroll; runtime selector and modal navigation reflow without hiding critical actions. |

Testing prohibitions:

- do not assert raw CSS values, class strings, screenshot bytes, generated output, or file existence as proxies for behavior;
- do not duplicate the same invariant at component, route, and E2E layers unless each layer proves a distinct failure mode;
- do not weaken a failing test to accommodate the old implementation;
- do not mock the runtime contract at final integration boundaries when a real local daemon or canonical API harness is available;
- do not add a new standalone regression file when the canonical suite already owns the invariant.

## 11. Visual contract matrix

The final bundle must include at least these states. The TechSpec may add states but must not remove them without a written reason.

### 11.1 Agents list

- default row view;
- card view;
- long agent name and category;
- invalid agent;
- unavailable runtime/provider;
- empty catalog;
- compact viewport.

### 11.2 Agent detail

- valid active agent, Overview;
- invalid active agent with diagnostics;
- loading shell;
- agent not found;
- Overview with zero sessions;
- Instructions lane;
- Configuration lane;
- Sessions lane with mixed states;
- runtime selector open;
- runtime mutation pending;
- runtime mutation error/conflict;
- compact viewport with wrapped header actions.

### 11.3 Settings

- Basics, clean;
- Runtime section with selector open;
- Instructions with long content;
- Access with populated tool constraints;
- MCP servers populated and empty;
- Danger zone;
- dirty state;
- field validation error;
- save failure;
- concurrent update conflict;
- nested deletion confirmation;
- compact/mobile modal navigation.

### 11.4 Evidence bundle structure

Use the canonical screenshot tooling, but the conceptual output must be equivalent to:

```text
visual-contract/
├── manifest.json
├── agent-detail-invalid-overview/
│   ├── reference.png
│   ├── implementation.png
│   ├── side-by-side.png
│   ├── diff.png
│   ├── comparison.json
│   └── review.md
└── ...one directory per required state
```

`review.md` must classify divergences as:

- **blocking structural:** topology, missing region, wrong hierarchy, wrong component anatomy, wrong metric, wrong interaction, or unsupported runtime claim;
- **blocking accessibility:** focus, naming, contrast, target size, keyboard access, or motion failure;
- **authorized runtime delta:** the reference implies behavior the daemon does not support and the TechSpec approves the truthful alternative;
- **non-blocking rendering variance:** antialiasing or minor platform rendering differences that do not affect structure.

Pixel similarity alone is insufficient. A perfectly aligned static selector that does not submit real runtime changes still fails.

## 12. Accessibility and responsive acceptance

### 12.1 Accessibility

The implementation must satisfy at least:

- one visible page H1 in the content detail header;
- correctly labeled tabs and tab panels;
- dialog name and description connected to the settings header;
- initial focus placed intentionally;
- focus trapped in modal and nested confirmation;
- focus returned to the triggering control;
- Escape closes only when safe or opens dirty confirmation;
- all icon-only controls have accessible names;
- status is conveyed by text, not color alone;
- error summary and field errors are programmatically associated;
- pending operations expose state without disabling recovery paths;
- keyboard operation for runtime search, provider rail, model list, and reasoning choices;
- reduced-motion behavior for transitions;
- minimum target sizes according to the canonical web contract.

### 12.2 Responsive behavior

The feature must be checked at the project's standard responsive widths. At minimum:

- the detail header moves actions below identity before truncating name/status;
- metrics reflow from four columns without horizontal scroll;
- tabs scroll or recompose predictably;
- main/aside content becomes a single prioritized column;
- settings navigation moves from left rail to compact horizontal/wrapped navigation;
- modal body retains exactly one intentional scroll region;
- footer actions remain reachable and do not cover fields;
- runtime selector uses its compact contract when necessary;
- tables provide a deliberate compact treatment rather than a squeezed desktop table.

## 13. Data, workspace isolation, and agent-manageability

### 13.1 Data classification

Classify each datum before implementation:

| Datum | Expected scope | Isolation requirement |
| --- | --- | --- |
| Agent definition | Workspace | Every read/update/delete includes workspace identity. |
| Runtime default | Workspace agent | Cache key and mutation identity include workspace and agent. |
| Session counts and runtime | Workspace agent aggregation | Store and API queries filter before aggregation. |
| Last activity | Workspace agent aggregation | Events from other workspaces cannot enter the result or SSE cache. |
| Provider availability | Usually operator/global plus workspace policy overlay | UI must distinguish installation/auth availability from workspace authorization. |
| Model catalog | Provider/global cache with policy-aware consumption | Cache reuse must not bypass workspace restrictions. |
| Settings draft | Browser-local route instance | Never written to shared cache before save. |

### 13.2 Agent-manageability

Any newly supported runtime-default operation must be manageable outside the UI through the existing public AGH surfaces. The implementation audit must answer:

- Which HTTP endpoint reads and updates the runtime default?
- Which UDS/CLI command exposes the same operation with structured output?
- Is there an `agh__*` native tool for the operation, and if so, do its descriptor, schema digest, risk flags, and capability gates change?
- How are conflicts and validation errors represented consistently across HTTP, UDS, CLI, and native tools?
- Does the official AGH skill teach the new or changed path?

If the operation already exists and only the UI begins consuming it, document “no contract change” with exact checked surfaces. Do not create a UI-only mutation path.

### 13.3 Extensibility and config lifecycle

The TechSpec must verify whether runtime selection interacts with:

- extension-provided providers;
- runtime-advertised configuration options;
- model-catalog refresh hooks;
- skills/capability gates;
- toolsets and bundles;
- bridge SDKs or MCP sidecars;
- `config.toml` defaults or policy restrictions.

The selector must consume registries and extension points. It must not hardcode a closed list of providers in the web feature.

## 14. AGH cross-surface impact audit

The implementation plan and completion report must include this audit with concrete evidence:

### Native tools

Likely impact depends on whether the agent runtime update contract changes. Check all `agh__*` agent-definition tools, toolsets, descriptors, I/O schemas, digests, risk flags, availability diagnostics, capability gates, and CLI/API fallbacks. If the backend contract is unchanged and the web only begins using it, record the exact tools inspected and why no update is required.

### Extensibility and hooks

Check provider and model registries, runtime-advertised options, extensions, hooks, skills/capabilities, tools/resources, bundles, bridge SDKs, MCP sidecars, and config lifecycle. The unified selector must remain registry-driven. Record any hook or capability event triggered by a default-runtime update.

### Workspace data isolation

Agent definition, runtime default, session metrics, and last activity are workspace-scoped. Prove `workspace_id` propagation through CLI/HTTP/UDS, core/store queries, web query keys, mutation invalidation, SSE/events, and caches. Include cross-workspace negative tests.

### Official AGH skill

Update `skills/agh/` if routes, tool IDs, CLI paths, runtime update semantics, provider/model/reasoning behavior, capabilities, or destructive settings behavior change. Otherwise name the inspected sections and explain why the public operating guidance remains accurate.

## 15. QA tracker impact

This redesign is user-visible and therefore cannot be declared a pure refactor.

Required tracker actions:

- reset existing agent list, detail, and settings scenarios to `untested` when their expected UI or behavior changes;
- add a content-addressed scenario for deep-linked settings overlay behavior if none exists;
- add a scenario for live runtime update success, conflict, and rollback if this is newly user-operable;
- add a scenario for workspace-isolated agent metrics if it is not already covered;
- add or update a destructive deletion scenario;
- do not retest these scenarios as part of merely flagging impact; the next QA cycle owns execution.

Any later real-scenario QA run must use an isolated lab, derive the web proxy target from its manifest, register long-lived PIDs, and finish with clean teardown evidence.

## 16. Validation sequence

Run validation in this order to get fast feedback without weakening the final gate:

1. component and route tests for the touched web systems;
2. Turborepo lint, typecheck, and test lanes for `web` and `packages/ui` as applicable;
3. Storybook or canonical component validation for changed shared primitives;
4. deterministic visual-contract capture for all required states;
5. structural divergence review and remediation;
6. targeted browser/E2E behavior for routing, modal focus, runtime mutation, and responsive states;
7. codegen and drift check only if canonical token or generated-contract sources changed;
8. deslop review of the final diff;
9. `make verify` exactly once as the completion gate;
10. implementation peer review until SHIP;
11. QA teardown verification.

No completion claim is valid if scoped tests pass but the visual bundle is incomplete, or if the visual bundle passes while runtime behavior is mocked or false.

## 17. Risks and mitigations

| Risk | Why it matters | Mitigation |
| --- | --- | --- |
| The current reference differs from the version originally approved. | Implementers may match a later prototype accidentally. | Freeze and approve one version; record superseded hashes and authorized deltas. |
| Modal routing causes data refetch or background reset. | The overlay loses context and feels like a page replacement. | Keep the detail shell mounted; test query continuity and browser history. |
| Live runtime update races with settings draft. | One surface can silently overwrite the other. | Define version-aware mutation and explicit synchronization rules. |
| Runtime catalog options change while the selector is open. | Selected values may become invalid. | Revalidate at commit; refresh options; preserve server truth on rejection. |
| Shared selector becomes a god component. | Cross-surface reuse can centralize too many concerns. | Split data derivation, trigger, popup, model list, provider rail, reasoning control, and mutation adapters by responsibility. |
| Visual parity work overfits static fixture data. | Real long names, empty states, and failures regress. | Capture a state matrix with boundary content and real behavioral tests. |
| Global token changes create unrelated regressions. | A local composition problem becomes a system-wide visual churn. | Default to no token change; require semantic multi-consumer evidence. |
| Metrics are computed client-side from paginated sessions. | Counts and duration become incomplete or workspace-leaky. | Use canonical backend aggregation or a proven complete data source. |
| Review accepts unresolved evidence as follow-up again. | The same governance failure repeats. | Make provenance and zero blocking structural mismatches explicit P0 gates. |
| Old and new settings paths coexist. | Behavior, tests, and CSS drift immediately. | Hard-delete old topology and compatibility branches in the same change. |

## 18. Definition of done

The remediation is complete only when all statements below are true.

### Product and interaction

- Agent detail has a content `DetailHeader` distinct from the route top bar.
- Overview shows Active, Runtime, Failed, and Last activity with documented semantics.
- Settings is a deep-linkable modal over the mounted detail context.
- Settings supports clean, dirty, validation-error, save-failure, conflict, and delete-confirmation states.
- Provider · Model · Reasoning uses the shared, truthful runtime selector.
- Live runtime updates are version-aware and recover correctly on failure.
- Agent rows and cards preserve semantic typography and metadata roles.
- Responsive and keyboard behavior are production-ready.

### Architecture

- Generic primitives come from `@agh/ui`; agent-specific compositions remain in the agent system.
- No duplicate selector or obsolete full-page settings path remains.
- No production file grows into a multi-responsibility god file.
- Workspace scope is proven across data, cache, mutation, event, and aggregation boundaries.
- Public operations remain manageable through agent-facing structured surfaces.

### Specification and evidence

- A new TechSpec supersedes the conflicting prior decisions.
- Approved references are frozen, hashed, and rendered.
- Every required visual state has a complete validated evidence bundle.
- `review.md` reports zero unresolved blocking structural or accessibility divergence.
- Runtime-truth deltas are explicit and approved.
- Workflow changes prevent ignored or mutable references from silently entering future specs.

### Quality and delivery

- Tests are placed by invariant and owning layer.
- Scoped frontend gates pass.
- QA tracker impact is committed.
- Docs and official skill impact are resolved.
- `make verify` passes once at the final gate.
- Deslop review is complete.
- External implementation review reaches SHIP.
- QA teardown reports clean, with no daemon, browser, watcher, or dev server left running.

## 19. Recommended execution order

The safe dependency order is:

1. freeze reference versions and approve the new TechSpec;
2. harden reference provenance in the spec workflow;
3. audit and, only if necessary, extend shared primitives;
4. establish the route shell that can host an overlay;
5. implement the detail hierarchy and canonical metrics;
6. implement the settings modal and persistence semantics;
7. integrate live runtime mutation;
8. correct list/card typography and shared agent presentation;
9. complete tests and the full visual state matrix;
10. update QA tracking, docs, and official skill;
11. run the final gates and peer-review loop.

Do not begin with token changes or local CSS polish. Those actions can make the wrong architecture look more polished while preserving the root defect.

## 20. Final implementation handoff

The engineer executing this plan should begin by producing four short artifacts inside the new TechSpec directory:

1. `reference-manifest.yaml` — immutable files, hashes, states, normative scope, and authorized deltas;
2. `architecture.md` or the corresponding TechSpec section — route topology, component ownership, mutations, cache behavior, and delete targets;
3. `_tests.md` — invariant ownership and canonical suites;
4. `visual-state-matrix.md` — reference/implementation pairs and acceptance status.

These artifacts prevent the implementation from turning a visual reference into a vague request for “better spacing.” The actual objective is stronger: preserve the approved information architecture, make every control truthful and operable, reuse the canonical system, and prove the result across behavior and appearance.

