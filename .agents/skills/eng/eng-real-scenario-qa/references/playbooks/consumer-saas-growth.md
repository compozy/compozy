# Lumen Notes — Activation A/B Growth Sprint

## Company

**Lumen Notes** is a consumer SaaS for everyday note-taking on the verge of a big activation push. The team is running a two-week growth sprint focused on first-week retention: ship two A/B variants of the post-signup landing, instrument event tracking, segment lapsed users, and ship a lifecycle email triggered on day-3. Marketing is impatient; data science is paranoid; engineering is short-staffed. A tracking event has been silently dropping for the last 24 hours and nobody has noticed yet.

Stress profile: many small artifacts (TS modules, SQL migration, lifecycle email TSX, A/B variant pages), heavy data-science cross-checking, and a real silent telemetry bug embedded in the seeded state.

## Operator persona

**Priya Joshi — Head of Growth at Lumen Notes**. Voice: outcomes-first, short sentences, references the activation funnel, the experiment ledger, and the data-science weekly.

## Workspaces (4)

| id | name | purpose |
|---|---|---|
| ws_growth_lab | growth-lab | Experiment design, A/B variant ownership, decision memos |
| ws_product_design | product-design | Onboarding TSX surfaces, lifecycle email TSX templates |
| ws_data_science | data-science | Event tracking, segmentation SQL, statistical readouts |
| ws_lifecycle_marketing | lifecycle-marketing | Lifecycle automation, email cadence, copy review |

## Agents (7)

| id | role | workspace |
|---|---|---|
| growth-pm-agent | Growth PM | ws_growth_lab |
| experiment-engineer-agent | Experiment Engineer | ws_growth_lab |
| product-designer-agent | Product Designer | ws_product_design |
| frontend-engineer-agent | Frontend Engineer | ws_product_design |
| data-scientist-agent | Data Scientist | ws_data_science |
| analytics-engineer-agent | Analytics Engineer | ws_data_science |
| lifecycle-marketer-agent | Lifecycle Marketer | ws_lifecycle_marketing |

## Channels (4)

| id | purpose | primary workspace |
|---|---|---|
| growth-room | Sprint coordination, ship/hold calls, experiment decisions | ws_growth_lab |
| design-review | TSX surface review, copy alignment | ws_product_design |
| data-watch | Event tracking, dashboard checks, segment readouts | ws_data_science |
| lifecycle-cadence | Email send timing, copy review, suppression rules | ws_lifecycle_marketing |

## Disruption probe seeds

- **silent_event_drop** at minute 6 — overwrite a knowledge file with the previous day's event volume showing the activation event went to zero.
- **variant_assignment_skew** at minute 14 — channel message reporting variant B is getting 70/30 of traffic (assignment bug).
- **lifecycle_send_misfire** at minute 20 — task event recording an email send to a suppressed segment.

---

## Canonical machine-readable spec

```json
{
  "schema_version": 1,
  "playbook_ref": "consumer-saas-growth",
  "company": {
    "name": "Lumen Notes",
    "short_name": "Lumen",
    "tagline": "Two-week activation sprint to lift first-week retention",
    "stage": "Seed extension",
    "industry": "consumer-saas-productivity"
  },
  "operator_persona": {
    "role": "Head of Growth",
    "name": "Priya Joshi",
    "voice_guidelines": "Outcomes-first, short sentences. Reference the activation funnel, the experiment ledger, the data-science weekly, and the suppression rules. Never use evaluation or testing language."
  },
  "kickoff_brief": "Priya here. Two-week activation sprint kicks off now. Open work: ship variant A and variant B of the post-signup landing (TSX), instrument the activation event tracking module so it actually fires on first save, write the segmentation SQL for day-7 lapsed users, ship the day-3 lifecycle email TSX template, and write the experiment decision memo with the success metrics in advance. Data science owns the readout cadence; lifecycle marketing owns the suppression rules. The activation event has been quiet — confirm it is firing before we light the variants. Coordinate in growth-room and your domain channels; request review where the variant copy will affect the activation rate.",
  "workspaces": [
    { "id": "ws_growth_lab", "name": "growth-lab", "purpose": "Experiment design, A/B variant ownership, decision memos", "knowledge_files": ["workspace/experiment-ledger.md"] },
    { "id": "ws_product_design", "name": "product-design", "purpose": "Onboarding TSX surfaces, lifecycle email TSX templates" },
    { "id": "ws_data_science", "name": "data-science", "purpose": "Event tracking, segmentation SQL, statistical readouts", "knowledge_files": ["workspace/event-volume-yesterday.md"] },
    { "id": "ws_lifecycle_marketing", "name": "lifecycle-marketing", "purpose": "Lifecycle automation, email cadence, copy review" }
  ],
  "agents": [
    {
      "id": "growth-pm-agent",
      "role": "Growth PM",
      "persona": "Growth PM. Owns sprint sequence and experiment decisions.",
      "system_prompt": "You are the Growth PM at Lumen Notes. Hold the sprint sequence in growth-room, decide experiment ship/hold based on activation funnel evidence, and unblock owners. Reference the experiment ledger and the activation funnel definition before any decision.",
      "workspace": "ws_growth_lab"
    },
    {
      "id": "experiment-engineer-agent",
      "role": "Experiment Engineer",
      "persona": "Experiment Engineer. Owns the A/B assignment module and the success-metric guardrails.",
      "system_prompt": "You are the Experiment Engineer at Lumen Notes. Author the assignment TS module that splits traffic 50/50 between variant A and variant B and the success-metric guardrails module. Block experiment launch if the activation event tracking is not firing.",
      "workspace": "ws_growth_lab"
    },
    {
      "id": "product-designer-agent",
      "role": "Product Designer",
      "persona": "Product Designer. Owns variant copy direction and visual coherence.",
      "system_prompt": "You are the Product Designer at Lumen Notes. Direct the copy and visual choice for variant A and variant B. Variant A is the control (current copy); variant B is the empathy variant. Coordinate with the frontend engineer on design-review channel.",
      "workspace": "ws_product_design"
    },
    {
      "id": "frontend-engineer-agent",
      "role": "Frontend Engineer",
      "persona": "Frontend Engineer. Builds the variant TSX pages and the lifecycle email template.",
      "system_prompt": "You are the Frontend Engineer at Lumen Notes. Ship variant A (control) and variant B (empathy) of the post-signup landing as TSX pages; ship the day-3 lifecycle email as a TSX template. Hand off the landing variants to the product designer on design-review and the lifecycle email to the lifecycle marketer on lifecycle-cadence.",
      "workspace": "ws_product_design"
    },
    {
      "id": "data-scientist-agent",
      "role": "Data Scientist",
      "persona": "Data Scientist. Owns success metrics and the activation funnel definition.",
      "system_prompt": "You are the Data Scientist at Lumen Notes. Confirm the activation event is firing before any variant launches; if you see anomalous event volume, post immediately to data-watch and block the experiment from growth-room. Define success metrics in advance, not after the experiment ends.",
      "workspace": "ws_data_science"
    },
    {
      "id": "analytics-engineer-agent",
      "role": "Analytics Engineer",
      "persona": "Analytics Engineer. Owns the segmentation SQL and the activation event tracking module.",
      "system_prompt": "You are the Analytics Engineer at Lumen Notes. Author the segmentation SQL migration for day-7 lapsed users and the activation event tracking TS module. Maintain unit tests for both. Coordinate with the data scientist on data-watch.",
      "workspace": "ws_data_science"
    },
    {
      "id": "lifecycle-marketer-agent",
      "role": "Lifecycle Marketer",
      "persona": "Lifecycle Marketer. Owns the day-3 send and suppression rules.",
      "system_prompt": "You are the Lifecycle Marketer at Lumen Notes. Hold the day-3 lifecycle send timing and suppression rules. Never send to suppressed segments. Coordinate lifecycle copy with the frontend engineer on lifecycle-cadence and review the lifecycle email TSX before scheduling.",
      "workspace": "ws_lifecycle_marketing"
    }
  ],
  "channels": [
    { "id": "growth-room", "purpose": "Sprint coordination, ship/hold calls, experiment decisions", "primary_workspace": "ws_growth_lab" },
    { "id": "design-review", "purpose": "TSX surface review, copy alignment", "primary_workspace": "ws_product_design" },
    { "id": "data-watch", "purpose": "Event tracking, dashboard checks, segment readouts", "primary_workspace": "ws_data_science" },
    { "id": "lifecycle-cadence", "purpose": "Email send timing, copy review, suppression rules", "primary_workspace": "ws_lifecycle_marketing" }
  ],
  "knowledge_files": [
    {
      "path": "global/operator-style.md",
      "content": "# Operator narrative style\n\nOutcomes-first, short sentences. Reference the activation funnel, the experiment ledger, the data-science weekly, the suppression rules. Never use evaluation or testing language.\n"
    },
    {
      "path": "global/activation-funnel.md",
      "content": "# Activation funnel\n\nSignup -> first save -> third save in week one. The activation event 'first_save' is the single load-bearing event. If it stops firing, the funnel readout is meaningless. Always confirm event volume before reading conversion deltas.\n"
    },
    {
      "path": "global/suppression-rules.md",
      "content": "# Suppression rules\n\nNever email: paid users, support-escalation contacts in the last 7 days, users in the active-investigation segment, users with marketing-opt-out flag. Lifecycle marketer is responsible for enforcing these on every send.\n"
    },
    {
      "path": "workspace/experiment-ledger.md",
      "content": "# Experiment ledger\n\n## Open\n- act-2026-04 — Variant A vs B post-signup landing. Owner: Priya. Success metric: first_save rate within 24h. Min sample: 4,000 per arm.\n\n## Past\n- act-2026-02 — Day-3 lifecycle email subject test. Verdict: empathy subject won by 6.2%.\n- act-2026-03 — Onboarding step reduction. Verdict: no significant lift; held current flow.\n"
    },
    {
      "path": "workspace/event-volume-yesterday.md",
      "content": "# Event volume yesterday\n\n- signup: 12,430 events\n- first_save: 7,812 events\n- third_save: 3,019 events\n\nLast updated 24h ago. Refresh before any experiment launch.\n"
    }
  ],
  "open_tasks": [
    {
      "title": "Variant A — control TSX page",
      "description": "Post-signup landing TSX page using current copy direction.",
      "owner_agent": "frontend-engineer-agent",
      "deliverable_type": "tsx_page",
      "deliverable_path_hint": "ws_product_design/onboarding/variant-a.tsx",
      "review_required_by": "product-designer-agent",
      "channel": "design-review"
    },
    {
      "title": "Variant B — empathy TSX page",
      "description": "Post-signup landing TSX page using empathy copy direction; same component shape as variant A.",
      "owner_agent": "frontend-engineer-agent",
      "deliverable_type": "tsx_page",
      "deliverable_path_hint": "ws_product_design/onboarding/variant-b.tsx",
      "review_required_by": "product-designer-agent",
      "channel": "design-review"
    },
    {
      "title": "Day-3 lifecycle email TSX template",
      "description": "TSX email template for day-3 send; consumed by lifecycle-marketer.",
      "owner_agent": "frontend-engineer-agent",
      "deliverable_type": "tsx_component",
      "deliverable_path_hint": "ws_product_design/lifecycle/day3-email.tsx",
      "review_required_by": "lifecycle-marketer-agent",
      "channel": "lifecycle-cadence"
    },
    {
      "title": "A/B assignment TS module",
      "description": "Module that splits incoming users 50/50; deterministic on user id; emits assignment event.",
      "owner_agent": "experiment-engineer-agent",
      "deliverable_type": "ts_module",
      "deliverable_path_hint": "ws_growth_lab/assignment/split.ts",
      "review_required_by": "data-scientist-agent",
      "channel": "growth-room"
    },
    {
      "title": "Activation event tracking TS module",
      "description": "Module that fires the first_save activation event on first successful save; consumed by the funnel reader.",
      "owner_agent": "analytics-engineer-agent",
      "deliverable_type": "ts_module",
      "deliverable_path_hint": "ws_data_science/tracking/activation.ts",
      "review_required_by": "data-scientist-agent",
      "channel": "data-watch"
    },
    {
      "title": "Day-7 lapsed user segmentation SQL",
      "description": "SQL migration creating the lapsed-user segment view; referenced by lifecycle-marketer.",
      "owner_agent": "analytics-engineer-agent",
      "deliverable_type": "sql_migration",
      "deliverable_path_hint": "ws_data_science/segments/day7-lapsed.sql",
      "review_required_by": "data-scientist-agent",
      "channel": "data-watch"
    },
    {
      "title": "A/B assignment unit tests",
      "description": "Vitest suite covering deterministic split, edge cases on missing user id, and event emission.",
      "owner_agent": "experiment-engineer-agent",
      "deliverable_type": "ts_test",
      "deliverable_path_hint": "ws_growth_lab/assignment/split.test.ts",
      "review_required_by": "data-scientist-agent",
      "channel": "growth-room"
    },
    {
      "title": "Activation tracking unit tests",
      "description": "Vitest suite covering activation event firing once per user, idempotency, and absence of double-counting.",
      "owner_agent": "analytics-engineer-agent",
      "deliverable_type": "ts_test",
      "deliverable_path_hint": "ws_data_science/tracking/activation.test.ts",
      "review_required_by": "data-scientist-agent",
      "channel": "data-watch"
    },
    {
      "title": "Experiment decision memo",
      "description": "Pre-experiment decision memo: success metric, MDE, sample size, ship rule, hold rule.",
      "owner_agent": "growth-pm-agent",
      "deliverable_type": "spec_md",
      "deliverable_path_hint": "ws_growth_lab/decisions/act-2026-04-memo.md",
      "review_required_by": "data-scientist-agent",
      "channel": "growth-room"
    },
    {
      "title": "Lifecycle send runbook",
      "description": "Runbook documenting the day-3 send: trigger, suppression checks, rollback, rollback owner.",
      "owner_agent": "lifecycle-marketer-agent",
      "deliverable_type": "runbook_md",
      "deliverable_path_hint": "ws_lifecycle_marketing/runbooks/day3-send.md",
      "review_required_by": "growth-pm-agent",
      "channel": "lifecycle-cadence"
    },
    {
      "title": "Tracking outage incident decision",
      "description": "Live decision recorded when the silent event drop disruption fires: investigate / hold experiment / continue.",
      "owner_agent": "data-scientist-agent",
      "deliverable_type": "spec_md",
      "deliverable_path_hint": "ws_data_science/decisions/event-outage-decision.md",
      "review_required_by": "growth-pm-agent",
      "channel": "data-watch"
    }
  ],
  "required_deliverables": {
    "tsx_page": 2,
    "tsx_component": 1,
    "ts_module": 2,
    "ts_test": 2,
    "sql_migration": 1,
    "runbook_md": 1,
    "spec_md": 1
  },
  "required_collaboration": {
    "peer_messages_min": 12,
    "review_cycles_min": 3,
    "disagreements_resolved_min": 1,
    "channels_active_min": 3
  },
  "disruption_probe_seeds": [
    {
      "type": "silent_event_drop",
      "seed_at_minute": 6,
      "delivery": "knowledge_file",
      "expected_recovery": "Data scientist reads the updated event-volume knowledge file, posts the anomaly to data-watch within 5 minutes, and blocks experiment launch from growth-room until the analytics engineer confirms the activation tracking module is wired."
    },
    {
      "type": "variant_assignment_skew",
      "seed_at_minute": 14,
      "delivery": "channel_message",
      "expected_recovery": "Experiment engineer reproduces the skew, fixes the deterministic split, posts the verdict to growth-room within 10 minutes."
    },
    {
      "type": "lifecycle_send_misfire",
      "seed_at_minute": 20,
      "delivery": "task_event",
      "expected_recovery": "Lifecycle marketer pauses the day-3 send, audits the suppression list, posts the next-step decision to lifecycle-cadence within 8 minutes."
    }
  ]
}
```
