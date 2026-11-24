import { LOCAL_COMPUTER_ID, normalizeComputerId, type ComputerId } from "@/lib/computer-scope";

import {
  ROOT_ENVIRONMENT_TARGET,
  type SessionEnvironmentTarget,
} from "./session-environment-target";

export interface SessionCreateDialogDraft {
  agentName: string;
  workspaceId: string;
  sessionName: string;
  /** Target computer where the session will be created; immutable after send (UT-082). */
  computerId: ComputerId;
  /** Locked once the draft has been sent (UT-082). */
  sent: boolean;
  /** Where the session will run. Worktrees belong to one workspace, so this resets with it. */
  environment: SessionEnvironmentTarget;
  /** Session folder the new session files into; empty for the workspace root. */
  folderId: string;
}

/** Runtime's canonical built-in agent for a new session. */
export const DEFAULT_SESSION_AGENT_NAME = "general";

/** Fields hidden by Simple mode and reset when the operator leaves Advanced. */
export const ADVANCED_DEFAULTS = {
  environment: ROOT_ENVIRONMENT_TARGET,
};

export const EMPTY_SESSION_CREATE_DRAFT: SessionCreateDialogDraft = {
  agentName: DEFAULT_SESSION_AGENT_NAME,
  workspaceId: "",
  sessionName: "",
  computerId: LOCAL_COMPUTER_ID,
  sent: false,
  folderId: "",
  ...ADVANCED_DEFAULTS,
};

/**
 * Changes the draft's target computer before send. Once the draft has been sent,
 * the target computer is immutable and `setSessionDraftComputer` is a no-op (UT-082).
 */
export function setSessionDraftComputer(
  current: SessionCreateDialogDraft,
  computerId: ComputerId
): SessionCreateDialogDraft {
  if (current.sent) return current;
  const nextComputerId = normalizeComputerId(computerId);
  if (current.computerId === nextComputerId) return current;
  return {
    ...current,
    computerId: nextComputerId,
    workspaceId: "",
    environment: ROOT_ENVIRONMENT_TARGET,
    folderId: "",
  };
}

/** Marks a draft as sent so its target computer can no longer change (UT-082). */
export function markSessionDraftSent(
  current: SessionCreateDialogDraft
): SessionCreateDialogDraft {
  return current.sent ? current : { ...current, sent: true };
}

/** Preserve the selected workspace while changing the session agent. */
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
    computerId: current.computerId,
    sent: current.sent,
  };
}
