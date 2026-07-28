import type { AgentPayload } from "../types";
import { storyAgentNames } from "@/storybook/fintech-scenario";

/** Stable 64-char hex stand-in for AgentDefinitionDigest (not a real hash). */
export const FIXTURE_AGENT_DEFINITION_DIGEST =
  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";

export const agentFixtures: AgentPayload[] = [
  {
    name: storyAgentNames.cto,
    provider: "claude",
    prompt:
      "Own launch command, arbitrate technical risk, and produce concise operator-ready briefings for the 18:30 UTC cutover.",
    origin: "global",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.cfo,
    provider: "claude",
    prompt:
      "Track burn, launch revenue pacing, reserve exposure, and operator-visible finance decisions across launch week.",
    origin: "global",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.product,
    provider: "gemini",
    prompt:
      "Maintain the launch checklist, align product decisions across teams, and keep the operator on the highest-leverage next step.",
    origin: "global",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.frontend,
    provider: "codex",
    model: "gpt-5.4",
    prompt:
      "Validate launch UI states, patch landing-page regressions, and summarize any customer-facing risk before ship.",
    origin: "global",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.marketing,
    provider: "gemini",
    prompt:
      "Coordinate launch timing, CRM sends, ad spend windows, and campaign sequencing across the go-to-market team.",
    origin: "global",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.copywriter,
    provider: "claude",
    prompt:
      "Polish launch headlines, claims, emails, and support macros so every operator-facing draft is publishable.",
    origin: "global",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.fraud,
    provider: "claude",
    prompt:
      "Investigate suspicious payout holds, reserve anomalies, and launch-day risk spikes before operators approve merchant actions.",
    origin: "global",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.support,
    provider: "claude",
    prompt:
      "Handle merchant escalations, cluster repeat issues, and prepare the next support reply with customer-safe language.",
    origin: "global",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.compliance,
    provider: "qwen-code",
    model: "qwen3.6-plus",
    prompt:
      "Check KYB evidence, sanctions flags, and claims compliance before a launch-room decision is finalized.",
    origin: "global",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.release,
    provider: "codex",
    model: "gpt-5.4",
    prompt:
      "Own release verification, canary promotion, rollback guardrails, and cross-system launch readiness updates.",
    origin: "global",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.platform,
    provider: "codex",
    model: "gpt-5.4",
    prompt:
      "Investigate webhook failures, partner API drift, and rollout bottlenecks across the checkout platform.",
    origin: "global",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
];

export const primaryAgentFixture: AgentPayload = agentFixtures[0];
