import { isReasoningEffort, type ReasoningEffort } from "@/lib/api-contract";
import { resolveAgentRuntimeValue, type AgentPayload } from "@/systems/agent";
import type { SessionProviderOption } from "@/systems/workspace";

import type { NetworkParticipationStrategy } from "@/systems/network";

export interface SessionCreateDialogDraft {
  agentName: string;
  workspaceId: string;
  sessionName: string;
  prompt: string;
  workspacePath: string;
  providerOverride: string;
  modelOverride: string;
  reasoningEffort: ReasoningEffort | "";
  networkParticipationMode: "local" | "live";
  networkChannelId: string;
  networkChannelStrategy: NetworkParticipationStrategy | "";
}

/** Fields hidden by Simple mode and reset when the operator leaves Advanced. */
export const ADVANCED_DEFAULTS = {
  workspacePath: "",
  networkParticipationMode: "local" as const,
  networkChannelId: "",
  networkChannelStrategy: "" as NetworkParticipationStrategy | "",
};

/** Runtime choices are visible in the composer but reset with agent/workspace identity. */
export const RUNTIME_OVERRIDE_DEFAULTS = {
  providerOverride: "",
  modelOverride: "",
  reasoningEffort: "" as ReasoningEffort | "",
};

export const EMPTY_SESSION_CREATE_DRAFT: SessionCreateDialogDraft = {
  agentName: "",
  workspaceId: "",
  sessionName: "",
  prompt: "",
  ...RUNTIME_OVERRIDE_DEFAULTS,
  ...ADVANCED_DEFAULTS,
};

/** Preserve same-workspace text while resetting selections derived from agent identity. */
export function applySessionAgentSelection(
  current: SessionCreateDialogDraft,
  nextAgentName: string,
  nextWorkspaceId: string
): SessionCreateDialogDraft {
  if (current.agentName === nextAgentName && current.workspaceId === nextWorkspaceId) {
    return current;
  }
  return {
    ...EMPTY_SESSION_CREATE_DRAFT,
    agentName: nextAgentName,
    workspaceId: nextWorkspaceId,
    prompt: current.workspaceId === nextWorkspaceId ? current.prompt : "",
  };
}

function pickDefaultProvider(
  agent: AgentPayload | undefined,
  options: SessionProviderOption[]
): string {
  if (options.length === 0) {
    return "";
  }
  const effectiveProvider = resolveAgentRuntimeValue(agent).provider;
  if (options.some(option => option.name === effectiveProvider)) {
    return effectiveProvider;
  }
  return "";
}

export function resolveSelectedProvider(
  agentName: string,
  providerOverride: string,
  agent: AgentPayload | undefined,
  options: SessionProviderOption[]
): string {
  if (providerOverride.length > 0 && options.some(option => option.name === providerOverride)) {
    return providerOverride;
  }
  if (agentName.trim().length === 0) {
    return "";
  }
  return pickDefaultProvider(agent, options);
}

export function normalizeEffort(effort: string): ReasoningEffort | "" {
  return effort === "" ? "" : isReasoningEffort(effort) ? effort : "";
}

type SessionWorkspacePathResolution =
  | { workspacePath: string }
  | { error: "Working path must be relative to the selected workspace." }
  | { error: "Working path must stay within the selected workspace." }
  | { error: "The selected workspace root must be an absolute path." };

/**
 * Resolves the dialog's relative working-path input into the absolute create-session contract.
 * The input cannot escape the workspace selected for its agent/provider population.
 */
export function resolveSessionWorkspacePath(
  workspaceRoot: string,
  workingPath: string
): SessionWorkspacePathResolution {
  const trimmedWorkingPath = workingPath.trim();
  if (trimmedWorkingPath.length === 0) {
    return { workspacePath: "" };
  }
  if (trimmedWorkingPath.startsWith("/")) {
    return { error: "Working path must be relative to the selected workspace." };
  }
  if (trimmedWorkingPath.split("/").includes("..")) {
    return { error: "Working path must stay within the selected workspace." };
  }

  const trimmedWorkspaceRoot = workspaceRoot.trim();
  if (!trimmedWorkspaceRoot.startsWith("/")) {
    return { error: "The selected workspace root must be an absolute path." };
  }

  const normalizedWorkspaceRoot =
    trimmedWorkspaceRoot === "/" ? "" : trimmedWorkspaceRoot.replace(/\/+$/, "");
  return { workspacePath: `${normalizedWorkspaceRoot}/${trimmedWorkingPath}` };
}

export function describeWorkspaceError(error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "Unable to load provider options for this workspace.";
}

export function describeError(fallback: string, error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}
