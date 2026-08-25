import type { AgentPayload } from "../types";
import { storyAgentNames } from "@/storybook/fintech-scenario";

/** Stable 64-char hex stand-in for AgentDefinitionDigest (not a real hash). */
export const FIXTURE_AGENT_DEFINITION_DIGEST =
  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";

/**
 * `description` is what another agent reads when choosing a specialist, so each
 * one says what the agent does *and what it hands back*. Two rows are shaped
 * deliberately: `triage`-style definitions may carry no description at all
 * (valid, and the roster must render the gap honestly), and a workspace
 * definition may shadow a global one of the same name.
 */
export const agentFixtures: AgentPayload[] = [
  {
    name: storyAgentNames.cto,
    provider: "claude",
    prompt:
      "Own launch command, arbitrate technical risk, and produce concise operator-ready briefings for the 18:30 UTC cutover.",
    description: "Arbitrates technical risk and returns a go/no-go with the reasoning.",
    origin: "global",
    scope: "global",
    shadowed: false,
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.cfo,
    provider: "claude",
    prompt:
      "Track burn, launch revenue pacing, reserve exposure, and operator-visible finance decisions across launch week.",
    description: "Reviews burn and reserve exposure and returns a structured finance read.",
    origin: "global",
    scope: "global",
    shadowed: false,
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.product,
    provider: "gemini",
    prompt:
      "Maintain the launch checklist, align product decisions across teams, and keep the operator on the highest-leverage next step.",
    description: "Maps a product area and returns its entry points and open decisions.",
    origin: "global",
    scope: "global",
    shadowed: false,
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.frontend,
    provider: "codex",
    model: "gpt-5.4",
    prompt:
      "Validate launch UI states, patch landing-page regressions, and summarize any customer-facing risk before ship.",
    description: "Checks launch UI states and returns customer-facing regressions with severity.",
    origin: "global",
    scope: "global",
    shadowed: false,
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.marketing,
    provider: "gemini",
    prompt:
      "Coordinate launch timing, CRM sends, ad spend windows, and campaign sequencing across the go-to-market team.",
    description: "Sequences campaign timing and returns the send plan with its dependencies.",
    origin: "global",
    scope: "global",
    shadowed: false,
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.copywriter,
    provider: "claude",
    prompt:
      "Polish launch headlines, claims, emails, and support macros so every operator-facing draft is publishable.",
    // Definitions without a description stay valid; the roster shows the gap
    // rather than inventing a summary from the prompt.
    description: "",
    origin: "global",
    scope: "global",
    shadowed: false,
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.fraud,
    provider: "claude",
    prompt:
      "Investigate suspicious payout holds, reserve anomalies, and launch-day risk spikes before operators approve merchant actions.",
    description: "Investigates payout holds and returns findings with a recommended action.",
    origin: "global",
    scope: "global",
    shadowed: false,
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.support,
    provider: "claude",
    prompt:
      "Handle merchant escalations, cluster repeat issues, and prepare the next support reply with customer-safe language.",
    description: "Clusters merchant escalations and returns the next reply, customer-safe.",
    origin: "global",
    scope: "global",
    shadowed: false,
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.compliance,
    provider: "qwen-code",
    model: "qwen3.6-plus",
    prompt:
      "Check KYB evidence, sanctions flags, and claims compliance before a launch-room decision is finalized.",
    description: "Reviews KYB and claims evidence and returns structured findings with severity.",
    // A workspace definition shadowing a global twin of the same name: the
    // workspace copy is the one that runs, and the roster must say so.
    origin: "workspace",
    scope: "workspace",
    shadowed: false,
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.release,
    provider: "codex",
    model: "gpt-5.4",
    prompt:
      "Own release verification, canary promotion, rollback guardrails, and cross-system launch readiness updates.",
    description: "Verifies a release and returns readiness with the blocking items named.",
    origin: "global",
    scope: "global",
    // Inactive name collision: a workspace definition of the same name wins.
    shadowed: true,
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: storyAgentNames.platform,
    provider: "codex",
    model: "gpt-5.4",
    prompt:
      "Investigate webhook failures, partner API drift, and rollout bottlenecks across the checkout platform.",
    description: "Traces webhook and partner-API failures and returns the failing hop.",
    origin: "global",
    scope: "global",
    shadowed: false,
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
];

export const primaryAgentFixture: AgentPayload = agentFixtures[0];
