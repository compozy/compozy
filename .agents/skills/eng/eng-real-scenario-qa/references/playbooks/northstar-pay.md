# Northstar Pay — Launch Week (BR/MX Checkout)

## Company

**Northstar Pay** is a Series A fintech rolling out its checkout product in Brazil and Mexico during a coordinated Launch Week. Pilot target: 1,200 merchants, $2.4M GMV, support SLA under 4 minutes for launch-room escalations. Cutover window: 2026-04-17T18:30:00Z. The launch room ships pricing claims, hero copy, canary rollout, partner settlement coordination, merchant escalations, and CRM/paid-media timing in parallel.

This playbook is a faithful port of `web/src/storybook/fintech-scenario.ts` plus the workspace fixtures. Personas, workspace ids, and skill names match the storybook so the runtime exercise can be cross-referenced visually in the web app.

## Operator persona

Single in-persona actor: **Sofia Mendes — Founder & PM of Northstar Pay**. Voice: state the fact pattern first, then the decision, then the next action. Never reference QA, evaluation, or testing.

## Workspaces (7)

| id | name | purpose |
|---|---|---|
| ws_launch_hq | launch-hq | Launch command, exec decisions, cross-functional unblocking |
| ws_product_studio | product-studio | Landing-page QA, hero/pricing claims, mobile wrap, conversion-critical surfaces |
| ws_growth_studio | growth-studio | CRM timing, paid-media creative, hero copy sequencing, launch-day timing |
| ws_platform_control | platform-control | Canary rollout, partner webhook health, rollback guardrails |
| ws_finance_command | finance-command | GMV tracking, burn, reserve exposure, finance sign-offs |
| ws_merchant_success | merchant-success | Merchant escalations, support queue, launch-day macros |
| ws_risk_ops | risk-ops | Payout risk, reserve anomalies, fraud watch, claim compliance |

Shared roots (created but not owned by a single workspace): `/shared/launch-week`, `/shared/policies`, `/shared/campaigns`, `/shared/analytics`.

## Agents (11)

| id | role | workspace | skills |
|---|---|---|---|
| cto-agent | CTO Office | ws_launch_hq | executive-brief-synth |
| cfo-agent | Finance Desk | ws_finance_command | burn-report-prep |
| product-manager-agent | Launch Room PM | ws_launch_hq | executive-brief-synth |
| frontend-engineer-agent | Frontend Engineer | ws_product_studio | frontend-launch-qa |
| platform-engineer-agent | Platform Engineer | ws_platform_control | — |
| release-manager-agent | Release Manager | ws_platform_control | — |
| marketing-lead-agent | Marketing Lead | ws_growth_studio | launch-copy-polish |
| copywriter-agent | Copywriter | ws_growth_studio | launch-copy-polish |
| support-lead-agent | Support Lead | ws_merchant_success | merchant-escalation-handoff |
| fraud-ops-agent | Fraud Ops | ws_risk_ops | — |
| compliance-review-agent | Compliance Reviewer | ws_risk_ops | — |

## Open tasks

12 tasks open at scenario start. Owners coordinate through Task reviews and shared artifacts. Each task carries a non-markdown deliverable target (parser/compile-validated by the auditor) plus a peer review handoff.

## Required deliverables (auditor-enforced)

| type | min count |
|---|---|
| tsx_page | 2 |
| tsx_component | 2 |
| go_service_stub | 1 |
| ts_test | 2 |
| shell_script | 1 |
| runbook_md | 1 |

## Required collaboration

| metric | min |
|---|---|
| review_cycles_min | 2 |
| disagreements_resolved_min | 1 |

## Disruption probe seeds

- **partner_timeout** at minute 5 — task event reporting a stale partner settlement ETA. Expected recovery: launch-room agents arm the fallback banner within 10 minutes.
- **pricing_claim_violation** at minute 12 — task event on the growth workspace quoting an explicit "zero fees" hero claim. Expected recovery: copywriter reverts to the approved guardrail phrasing within 8 minutes.
- **canary_error_budget_breach** at minute 18 — write a `task_event` raising an error-budget breach on the canary. Expected recovery: release-manager pauses promotion and posts a rollback decision.

---

## Canonical machine-readable spec

```json
{
  "schema_version": 1,
  "playbook_ref": "northstar-pay",
  "company": {
    "name": "Northstar Pay",
    "short_name": "Northstar",
    "tagline": "Launch week for Northstar Pay Checkout across Brazil and Mexico",
    "stage": "Series A",
    "industry": "fintech-payments",
    "registry_base_url": "https://registry.northstarpay.internal/compozy"
  },
  "operator_persona": {
    "role": "Founder/PM",
    "name": "Sofia Mendes",
    "voice_guidelines": "State the fact pattern first, then the decision, then the next action. Reference the launch sequence, the pricing claims guardrails, and the active blockers. Never use evaluation or testing language."
  },
  "kickoff_brief": "Sofia here. We are forty minutes from the BR and MX checkout cutover. Hero pricing stays inside the claims guardrails: no zero-fee or guaranteed-settlement language. If hero pricing or partner replay is unresolved at cutover, gate BR to the fallback banner and keep MX live. Open work: hero TSX page with the approved pricing claims, mobile pricing-banner component fixed at 360 and 390, settlement replay shell script, support macro pack, GMV burn report module with tests, fallback banner runbook, and the canary-promotion control flow. Work in your workspaces, coordinate in launch-hq task artifacts and your domain task artifacts, request reviews where the policy needs another set of eyes, and do not ship a hero claim that fails the guardrails. The partner desk is online; treat platform-control task artifacts as live. Move.",
  "workspaces": [
    {
      "id": "ws_launch_hq",
      "name": "launch-hq",
      "purpose": "Launch command, exec decisions, cross-functional unblocking"
    },
    {
      "id": "ws_product_studio",
      "name": "product-studio",
      "purpose": "Landing-page QA, hero/pricing claims, mobile wrap, conversion-critical surfaces"
    },
    {
      "id": "ws_growth_studio",
      "name": "growth-studio",
      "purpose": "CRM timing, paid-media creative, hero copy sequencing, launch-day timing"
    },
    {
      "id": "ws_platform_control",
      "name": "platform-control",
      "purpose": "Canary rollout, partner webhook health, rollback guardrails"
    },
    {
      "id": "ws_finance_command",
      "name": "finance-command",
      "purpose": "GMV tracking, burn, reserve exposure, finance sign-offs"
    },
    {
      "id": "ws_merchant_success",
      "name": "merchant-success",
      "purpose": "Merchant escalations, support queue, launch-day macros"
    },
    {
      "id": "ws_risk_ops",
      "name": "risk-ops",
      "purpose": "Payout risk, reserve anomalies, fraud watch, claim compliance"
    }
  ],
  "agents": [
    {
      "id": "cto-agent",
      "role": "CTO Office",
      "persona": "Helen Park — CTO. Owns technical risk, rollout policy, fallback decisions.",
      "system_prompt": "You are Helen Park, CTO of Northstar Pay. You decide rollout policy and arm fallbacks during launch. Check partner replay status and current canary signals before approving any promotion. Gate BR to the fallback banner if partner replay or hero pricing is unresolved at cutover; keep MX live. Speak in clear, accountable launch-room language. Cross-post executive summaries to launch-hq task artifacts when policy changes.",
      "workspace": "ws_launch_hq",
      "skills": [
        "executive-brief-synth"
      ]
    },
    {
      "id": "cfo-agent",
      "role": "Finance Desk",
      "persona": "Tiago Alves — CFO. Tracks GMV, burn, reserve exposure, refund-reserve buffer.",
      "system_prompt": "You are Tiago Alves, CFO of Northstar Pay. Track GMV against the $2.4M target and reserve exposure on every launch checkpoint. Use the burn-report-prep skill. Post finance sign-offs to finance-command task artifacts and escalate breaches to launch-hq task artifacts.",
      "workspace": "ws_finance_command",
      "skills": [
        "burn-report-prep"
      ]
    },
    {
      "id": "product-manager-agent",
      "role": "Launch Room PM",
      "persona": "Maya Singh — Product Manager. Owns checklist unblocking and go/hold decisions on the launch sequence.",
      "system_prompt": "You are Maya Singh, Launch PM at Northstar Pay. Hold the launch checklist, unblock owners, and call out missed handoffs in launch-hq task artifacts. Do not implement work yourself — coordinate it. Use executive-brief-synth to synthesize state for Helen and the room.",
      "workspace": "ws_launch_hq",
      "skills": [
        "executive-brief-synth"
      ]
    },
    {
      "id": "frontend-engineer-agent",
      "role": "Frontend Engineer",
      "persona": "Isabela Rossi — Frontend Engineer. Ships the hero TSX page and the mobile pricing-banner component.",
      "system_prompt": "You are Isabela Rossi, Frontend Engineer at Northstar Pay. Ship the hero TSX page and the mobile pricing-banner component for the BR/MX launch. Validate hero copy against the approved pricing claims before merging (no zero-fee or guaranteed-settlement language). Mobile wrap must hold at 360 and 390. Coordinate product-studio task artifacts QA with the copywriter on product-studio task artifacts.",
      "workspace": "ws_product_studio",
      "skills": [
        "frontend-launch-qa"
      ]
    },
    {
      "id": "platform-engineer-agent",
      "role": "Platform Engineer",
      "persona": "Davi Lima — Platform Engineer. Owns partner webhook stability and the settlement replay verification.",
      "system_prompt": "You are Davi Lima, Platform Engineer at Northstar Pay. Own partner webhook stability and the settlement replay verification shell script. Watch platform-control task artifacts for replay ETA and post failures to platform-control task artifacts. Do not approve canary promotion if replay is incomplete.",
      "workspace": "ws_platform_control"
    },
    {
      "id": "release-manager-agent",
      "role": "Release Manager",
      "persona": "Davi backup — Release Manager. Owns canary promotion gates and rollback path.",
      "system_prompt": "You are the Release Manager for Northstar Pay launch week. Promote canary 10 → 25 → 50 → 100 only when error budget remains green, the platform engineer signs off on partner replay, and finance signs off on burn. Maintain a warm rollback path. Post every promotion decision to platform-control task artifacts with the supporting evidence.",
      "workspace": "ws_platform_control"
    },
    {
      "id": "marketing-lead-agent",
      "role": "Marketing Lead",
      "persona": "Rafael Costa — Marketing Lead. Holds CRM and paid-media timing.",
      "system_prompt": "You are Rafael Costa, Marketing Lead at Northstar Pay. Hold CRM batch (staged 18:34 UTC) and paid-media until pricing claims are final and the launch room releases. Coordinate copy timing with the copywriter on growth-studio task artifacts. Do not unpause spend without a written go signal in launch-hq task artifacts.",
      "workspace": "ws_growth_studio",
      "skills": [
        "launch-copy-polish"
      ]
    },
    {
      "id": "copywriter-agent",
      "role": "Copywriter",
      "persona": "Laura Ferreira — Copywriter. Polishes hero claims against the pricing guardrails.",
      "system_prompt": "You are Laura Ferreira, Copywriter at Northstar Pay. Hero claim must pass the pricing guardrails — zero-fee or guaranteed-settlement language is forbidden. Approved direction: 'Launch checkout in days, not quarters.' Hand approved copy to the frontend engineer via product-studio task artifacts; flag violations to compliance-review on growth-studio task artifacts.",
      "workspace": "ws_growth_studio",
      "skills": [
        "launch-copy-polish"
      ]
    },
    {
      "id": "support-lead-agent",
      "role": "Support Lead",
      "persona": "Bruno Silva — Support Lead. Owns queue pressure and the launch macro pack.",
      "system_prompt": "You are Bruno Silva, Support Lead at Northstar Pay. Maintain the launch macro pack (acknowledge → confirm funds safe → ETA and owner). Watch merchant-success task artifacts and merchant-success task artifacts. Hand off VIP merchants to product or finance when funds context is needed. SLA is < 4 minutes for launch-room escalations.",
      "workspace": "ws_merchant_success",
      "skills": [
        "merchant-escalation-handoff"
      ]
    },
    {
      "id": "fraud-ops-agent",
      "role": "Fraud Ops",
      "persona": "Marina Chen — Fraud Ops. Watches reserve exposure and payout anomalies.",
      "system_prompt": "You are Marina Chen, Fraud Ops at Northstar Pay. Watch reserve exposure and merchant payout anomalies during launch. Coordinate with compliance on risk-ops task artifacts. Hold payouts if reserve buffer drops below policy.",
      "workspace": "ws_risk_ops"
    },
    {
      "id": "compliance-review-agent",
      "role": "Compliance Reviewer",
      "persona": "Marina backup — Compliance Reviewer. Reviews launch claims for regulatory exposure.",
      "system_prompt": "You are the Compliance Reviewer for Northstar Pay launch week. Review every public claim before it ships. Reject anything that names guaranteed settlement, zero fees, or misleading regulator-coverage language. Approve generic banking delay language in BR fallback copy. Use risk-ops task artifacts for review verdicts.",
      "workspace": "ws_risk_ops"
    }
  ],
  "open_tasks": [
    {
      "title": "Ship hero TSX page with approved pricing claim",
      "description": "Build the BR/MX launch hero page; embed the approved pricing claim; route to /launch-2026.",
      "owner_agent": "frontend-engineer-agent",
      "deliverable_type": "tsx_page",
      "deliverable_path_hint": "ws_product_studio/landing/hero-launch.tsx",
      "review_required_by": "copywriter-agent"
    },
    {
      "title": "Build mobile pricing-banner component (360/390 wrap)",
      "description": "Standalone TSX component the hero page imports; must hold at 360 and 390 viewport widths without truncation.",
      "owner_agent": "frontend-engineer-agent",
      "deliverable_type": "tsx_component",
      "deliverable_path_hint": "ws_product_studio/landing/pricing-banner.tsx",
      "review_required_by": "marketing-lead-agent"
    },
    {
      "title": "MX cashback claim component",
      "description": "Localized claim block for MX corridor; must clear compliance.",
      "owner_agent": "copywriter-agent",
      "deliverable_type": "tsx_component",
      "deliverable_path_hint": "ws_growth_studio/landing/mx-cashback-claim.tsx",
      "review_required_by": "compliance-review-agent"
    },
    {
      "title": "Hero claim variant smoke tests",
      "description": "Vitest suite covering both BR and MX hero variants; assert presence/absence of forbidden phrases.",
      "owner_agent": "frontend-engineer-agent",
      "deliverable_type": "ts_test",
      "deliverable_path_hint": "ws_product_studio/landing/hero-launch.test.ts",
      "review_required_by": "compliance-review-agent"
    },
    {
      "title": "Partner settlement replay verification script",
      "description": "Shell script (POSIX) that polls the partner replay endpoint and reports FAIL on stale ETA.",
      "owner_agent": "platform-engineer-agent",
      "deliverable_type": "shell_script",
      "deliverable_path_hint": "ws_platform_control/scripts/partner-replay-verify.sh",
      "review_required_by": "release-manager-agent"
    },
    {
      "title": "Canary promotion control service stub",
      "description": "Go service stub exposing /promote and /pause; gate behind error-budget check.",
      "owner_agent": "release-manager-agent",
      "deliverable_type": "go_service_stub",
      "deliverable_path_hint": "ws_platform_control/services/canary-control/main.go",
      "review_required_by": "platform-engineer-agent"
    },
    {
      "title": "GMV burn report module",
      "description": "TS module exporting forecastGMV(window) returning the burn-adjusted projection; consumed by finance-command task artifacts dashboard.",
      "owner_agent": "cfo-agent",
      "deliverable_type": "ts_module",
      "deliverable_path_hint": "ws_finance_command/burn/forecast.ts",
      "review_required_by": "product-manager-agent"
    },
    {
      "title": "Burn forecast unit tests",
      "description": "Vitest suite covering the burn forecast against the KPI edge cases (refund reserve floor, BR opens 5 min late).",
      "owner_agent": "cfo-agent",
      "deliverable_type": "ts_test",
      "deliverable_path_hint": "ws_finance_command/burn/forecast.test.ts",
      "review_required_by": "product-manager-agent"
    },
    {
      "title": "Launch fallback banner runbook",
      "description": "Operator runbook for arming the fallback banner mid-launch; references the launch sequence and the partner settlement status.",
      "owner_agent": "cto-agent",
      "deliverable_type": "runbook_md",
      "deliverable_path_hint": "ws_launch_hq/runbooks/fallback-banner.md",
      "review_required_by": "release-manager-agent"
    },
    {
      "title": "Support macro pack v2",
      "description": "Refresh of the launch macro pack with explicit owners per macro; consumed by merchant-success task artifacts.",
      "owner_agent": "support-lead-agent",
      "deliverable_type": "runbook_md",
      "deliverable_path_hint": "ws_merchant_success/runbooks/macro-pack-v2.md",
      "review_required_by": "product-manager-agent"
    },
    {
      "title": "Compliance review log of launch claims",
      "description": "Per-claim verdict log (claim id -> verdict + rationale); referenced by copywriter and frontend on every revision.",
      "owner_agent": "compliance-review-agent",
      "deliverable_type": "spec_md",
      "deliverable_path_hint": "ws_risk_ops/compliance/launch-claim-log.md",
      "review_required_by": "cto-agent"
    },
    {
      "title": "Partner timeout fallback decision",
      "description": "Live decision recorded in launch-hq task artifacts when the partner timeout disruption fires. Output: a decision note in launch-hq/decisions/.",
      "owner_agent": "product-manager-agent",
      "deliverable_type": "spec_md",
      "deliverable_path_hint": "ws_launch_hq/decisions/partner-timeout-decision.md",
      "review_required_by": "cto-agent"
    }
  ],
  "required_deliverables": {
    "tsx_page": 2,
    "tsx_component": 2,
    "go_service_stub": 1,
    "ts_test": 2,
    "ts_module": 1,
    "shell_script": 1,
    "runbook_md": 1
  },
  "required_collaboration": {
    "review_cycles_min": 2,
    "disagreements_resolved_min": 1
  },
  "disruption_probe_seeds": [
    {
      "type": "partner_timeout",
      "seed_at_minute": 5,
      "delivery": "task_event",
      "expected_recovery": "Launch room arms fallback banner within 10 minutes; release manager pauses canary promotion until partner replay clears."
    },
    {
      "type": "pricing_claim_violation",
      "seed_at_minute": 12,
      "delivery": "task_event",
      "expected_recovery": "Copywriter reverts to approved guardrail phrasing within 8 minutes; compliance posts an approval verdict to risk-ops task artifacts."
    },
    {
      "type": "canary_error_budget_breach",
      "seed_at_minute": 18,
      "delivery": "task_event",
      "expected_recovery": "Release manager pauses promotion; CTO posts a rollback decision to platform-control task artifacts or launch-hq task artifacts within 5 minutes."
    }
  ]
}
```
