import type { AgentPayload } from "@/systems/agent";

import type { SessionDerivePreview, SessionDeriveResult } from "../adapters/session-derive-api";
import type { SessionPayload } from "../types";
import { sessionFixtures } from "./fixtures";

/** A root user session with no lineage: the Continue source in stories and tests. */
export const deriveSourceSessionFixture: SessionPayload = {
  ...sessionFixtures[1]!,
  name: "Refactor flaky manager tests",
  agent_name: "claude",
  lineage: undefined,
};

/** `Carries over 42 messages · 61.3 KiB`. */
export const derivePreviewFixture: SessionDerivePreview = {
  message_count: 42,
  replay_bytes: 62_771,
  truncated: false,
  omitted_count: 0,
  source_turn_in_progress: false,
  native_fork_possible: false,
  cut: null,
  transcript: { epoch: 1, generation: 3, max_sequence: 318 },
};

/** `Carries over 30 of 42 messages · 128 KiB` + 12 omitted. */
export const deriveTruncatedPreviewFixture: SessionDerivePreview = {
  ...derivePreviewFixture,
  message_count: 30,
  replay_bytes: 131_072,
  truncated: true,
  omitted_count: 12,
};

const DEFINITION_DIGEST = "sha256:story-derive-agent";

function deriveAgent(
  name: string,
  provider: string,
  extra: Partial<AgentPayload> = {}
): AgentPayload {
  return {
    name,
    provider,
    prompt: `${name} agent`,
    origin: "workspace",
    workspace_id: deriveSourceSessionFixture.workspace_id,
    definition_digest: DEFINITION_DIGEST,
    ...extra,
  };
}

/** The source's agent first, so the dialog must skip it to preselect `codex`. */
export const deriveAgentsFixture: AgentPayload[] = [
  deriveAgent("claude", "claude"),
  deriveAgent("codex", "codex"),
];

/** An agent declaring two routes that read the same except for their account. */
export const deriveRoutedAgentsFixture: AgentPayload[] = [
  deriveAgent("claude", "claude", {
    fallback_chain: [
      { provider: "claude", model: "opus" },
      { provider: "claude", model: "opus", command_fingerprint: "sha256:3f9a7c21d0e4b5a6" },
    ],
  }),
  deriveAgent("codex", "codex"),
];

/** The child a continue creates: lineage `continue`, replay seed, first prompt staged. */
export function continuedSessionFixture(
  source: SessionPayload = deriveSourceSessionFixture,
  agentName = "codex"
): SessionPayload {
  return {
    ...source,
    id: "sess_continued_child",
    name: `${source.name ?? source.id} (continued)`,
    agent_name: agentName,
    state: "active",
    badge: "idle",
    lineage: {
      kind: "continue",
      parent_session_id: source.id,
      root_session_id: source.id,
      origin_agent_name: source.agent_name,
      spawn_depth: 0,
      auto_stop_on_parent: false,
      notify_creator: false,
      permission_policy: { tools: [], skills: [], mcp_servers: [], workspace_paths: [] },
      spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
    },
    derivation: {
      kind: "continue",
      source_session_id: source.id,
      seed: "replay",
      first_prompt: "staged",
    },
  };
}

export function deriveResultFixture(
  child: SessionPayload = continuedSessionFixture(),
  overrides: Partial<SessionDeriveResult["derived"]> = {}
): SessionDeriveResult {
  return {
    session: child,
    derived: {
      kind: "continue",
      source_session_id: child.lineage?.parent_session_id ?? "",
      child_session_id: child.id,
      origin_agent_name: child.lineage?.origin_agent_name ?? "",
      through_turn_id: "turn_story_41",
      seed: "replay",
      replay_message_count: 42,
      replay_bytes: 62_771,
      truncated: false,
      omitted_count: 0,
      source_turn_in_progress: false,
      first_prompt: "staged",
      replayed: false,
      ...overrides,
    },
  };
}
