import type {
  MemoryDecision,
  MemoryDecisionRevertResponse,
  MemoryDecisionsResponse,
  MemoryDeleteResponse,
  MemoryDreamTriggerResponse,
  MemoryEditResponse,
  MemoryHeader,
  MemoryReadResponse,
  MemorySearchResponse,
  MemoryWriteResponse,
} from "../types";
import { storyAgentNames, storyWorkspaceIds } from "@/storybook/fintech-scenario";

const MOD_TIME = "2026-04-17T17:30:00Z";
const DECIDED_AT = "2026-04-17T17:31:00Z";
const APPLIED_AT = "2026-04-17T17:31:01Z";

function memorySummary(
  summary: Pick<MemoryHeader, "filename" | "name" | "type"> &
    Partial<Omit<MemoryHeader, "filename" | "name" | "type">>
): MemoryHeader {
  return {
    mod_time: summary.mod_time ?? MOD_TIME,
    recall_count: summary.recall_count ?? 0,
    scope: summary.scope ?? "global",
    injection: summary.injection ?? true,
    system_managed: summary.system_managed ?? false,
    ...summary,
  };
}

function memoryRead(summary: MemoryHeader, content: string): MemoryReadResponse {
  return {
    memory: {
      summary,
      content,
    },
  };
}

const operatorStyleMemory = memorySummary({
  filename: "operator-style.md",
  name: "Operator Style",
  type: "user",
  description: "Guidance for calm, evidence-first operator communication during launch week.",
});

const launchBriefMemory = memorySummary({
  filename: "launch-week-brief.md",
  mod_time: "2026-04-17T16:50:00Z",
  name: "Launch Week Brief",
  type: "project",
  description:
    "Canonical launch narrative, KPI targets, cutover sequence, and cross-functional owners.",
});

const pricingGuardrailsMemory = memorySummary({
  filename: "pricing-claims-guardrails.md",
  mod_time: "2026-04-17T15:15:00Z",
  name: "Pricing Claims Guardrails",
  type: "reference",
  description:
    "Approved phrasing for pricing, fees, and guarantee language across ads, site copy, and support.",
  agent_name: storyAgentNames.copywriter,
  scope: "agent",
  agent_tier: "global",
  recall_count: 3,
  last_recalled_at: "2026-04-17T17:00:00Z",
});

const executiveRiskMemory = memorySummary({
  filename: "executive-risk-memo.md",
  mod_time: "2026-04-17T17:05:00Z",
  name: "Executive Risk Memo",
  type: "reference",
  description: "CTO and CFO notes on the remaining launch blockers and acceptable fallback paths.",
  agent_name: storyAgentNames.cto,
  scope: "workspace",
  workspace_id: storyWorkspaceIds.hq,
  recall_count: 2,
  staleness_banner: "Updated >7 days after last recall",
});

const supportMacroMemory = memorySummary({
  filename: "support-macro-pack.md",
  mod_time: "2026-04-17T16:25:00Z",
  name: "Support Macro Pack",
  type: "reference",
  description:
    "Launch-day support macros for merchant onboarding delays, failed payouts, and pricing questions.",
  agent_name: storyAgentNames.support,
  scope: "workspace",
  workspace_id: storyWorkspaceIds.hq,
});

const kpiGlossaryMemory = memorySummary({
  filename: "kpi-glossary.md",
  mod_time: "2026-04-17T14:45:00Z",
  name: "KPI Glossary",
  type: "reference",
  description:
    "Shared definitions for GMV, activation, reserve exposure, payback, and support SLA.",
  agent_name: storyAgentNames.cfo,
  scope: "workspace",
  workspace_id: storyWorkspaceIds.hq,
});

const ctoStyleMemory = memorySummary({
  filename: "cto-tone.md",
  mod_time: "2026-04-17T17:25:00Z",
  name: "CTO Tone",
  type: "user",
  description: "Direct, calm tone for CTO-facing summaries; lead with the next decision.",
  agent_name: storyAgentNames.cto,
  scope: "agent",
  agent_tier: "workspace",
  workspace_id: storyWorkspaceIds.hq,
  recall_count: 5,
  last_recalled_at: "2026-04-17T17:20:00Z",
});

export const memoryHeadersFixture: MemoryHeader[] = [
  operatorStyleMemory,
  launchBriefMemory,
  pricingGuardrailsMemory,
  executiveRiskMemory,
  supportMacroMemory,
  kpiGlossaryMemory,
  ctoStyleMemory,
];

export const memoryReadFixtures: Record<string, MemoryReadResponse> = {
  "operator-style.md": memoryRead(
    operatorStyleMemory,
    "# Operator Style\n\nState the fact pattern first, then the decision, then the next concrete action.\n"
  ),
  "launch-week-brief.md": memoryRead(
    launchBriefMemory,
    "# Launch Week Brief\n\nNorthstar Pay Checkout launches at 18:30 UTC across Brazil and Mexico with a target of 1,200 pilot merchants and $2.4M GMV in week one.\n"
  ),
  "pricing-claims-guardrails.md": memoryRead(
    pricingGuardrailsMemory,
    "# Pricing Claims Guardrails\n\nAvoid saying zero fees. Approved claim: predictable blended processing with launch-week credits for pilot merchants.\n"
  ),
  "executive-risk-memo.md": memoryRead(
    executiveRiskMemory,
    "# Executive Risk Memo\n\nRemaining blockers: partner settlement timeout visibility, launch-hero fallback copy, and support queue load above the four-minute SLA threshold.\n"
  ),
  "support-macro-pack.md": memoryRead(
    supportMacroMemory,
    "# Support Macro Pack\n\n1. Acknowledge the issue.\n2. Confirm whether funds are safe.\n3. Set the next ETA and owner.\n"
  ),
  "kpi-glossary.md": memoryRead(
    kpiGlossaryMemory,
    "# KPI Glossary\n\n- GMV: processed launch volume.\n- Activation: merchant completed the first successful checkout.\n- Reserve exposure: held funds awaiting fraud review.\n"
  ),
  "cto-tone.md": memoryRead(
    ctoStyleMemory,
    "# CTO Tone\n\nLead with the decision and the next owner. Never escalate without a concrete evidence ask.\n"
  ),
};

const editDecision: MemoryDecision = {
  id: "dec_edit_fixture",
  candidate_hash: "sha256:edit-candidate",
  op: "update",
  scope: "global",
  source: "rule",
  confidence: 0.92,
  decided_at: DECIDED_AT,
  applied_at: APPLIED_AT,
  target_filename: "operator-style.md",
  reason: "rule:exact-slug-collision",
  frontmatter: {
    filename: "operator-style.md",
    mod_time: MOD_TIME,
    name: "Operator Style",
    type: "user",
  },
};

const deleteDecision: MemoryDecision = {
  id: "dec_delete_fixture",
  candidate_hash: "sha256:delete-candidate",
  op: "delete",
  scope: "global",
  source: "rule",
  confidence: 1,
  decided_at: "2026-04-16T17:31:00Z",
  applied_at: "2026-04-16T17:31:01Z",
  target_filename: "operator-style.md",
  reason: "rule:explicit-delete",
  frontmatter: {
    filename: "operator-style.md",
    mod_time: MOD_TIME,
    name: "Operator Style",
    type: "user",
  },
};

const addDecision: MemoryDecision = {
  id: "dec_add_fixture",
  candidate_hash: "sha256:add-candidate",
  op: "add",
  scope: "global",
  source: "rule",
  confidence: 0.98,
  decided_at: "2026-04-17T18:31:00Z",
  applied_at: "2026-04-17T18:31:01Z",
  target_filename: "launch-brief.md",
  reason: "rule:new-entry",
  frontmatter: {
    filename: "launch-brief.md",
    mod_time: "2026-04-17T18:31:00Z",
    name: "Launch Brief",
    type: "project",
  },
};

export const memoryWriteFixture: MemoryWriteResponse = {
  applied: true,
  decision: editDecision,
};

export const memoryEditFixture: MemoryEditResponse = {
  applied: true,
  decision: editDecision,
};

export const memoryDeleteFixture: MemoryDeleteResponse = {
  applied: true,
  decision: deleteDecision,
};

export const memoryDecisionRevertFixture: MemoryDecisionRevertResponse = {
  decision: editDecision,
  reverted: true,
};

export const memoryDreamTriggerFixture: MemoryDreamTriggerResponse = {
  triggered: true,
  reason: "launch-week-refresh",
  dream: {
    id: "dream_fixture",
    status: "running",
    scope: "global",
    candidate_count: 0,
    promoted_count: 0,
    started_at: "2026-04-17T17:32:00Z",
  },
};

export const memorySearchFixture: MemorySearchResponse = {
  results: [
    {
      memory: pricingGuardrailsMemory,
      score: 0.91,
      snippet: "Approved phrasing for pricing, fees, and guarantee language…",
      why_recalled: ["fts5:exact-match", "scope:agent-global"],
    },
    {
      memory: launchBriefMemory,
      score: 0.74,
      snippet: "Canonical launch narrative, KPI targets, cutover sequence…",
      why_recalled: ["fts5:trigram"],
    },
  ],
  recall: {
    blocks: [
      {
        scope: "global",
        entries: [
          {
            id: "block_global_pricing",
            title: "Pricing Claims Guardrails",
            body: "Approved phrasing for pricing.",
            age_days: 0,
            mod_time: "2026-04-17T15:15:00Z",
            why_recalled: ["fts5:exact-match"],
          },
        ],
      },
    ],
    header: { content_hash: "sha256:recall", text: "Recall context for launch-week pricing." },
  },
};

export const memoryDecisionsFixture: MemoryDecisionsResponse = {
  decisions: [addDecision, editDecision, deleteDecision],
};
