import { useState } from "react";

import { isTerminalBroaderDecisionForbidden } from "@/systems/terminal/parts";
import { useActiveWorkspace } from "@/systems/workspace";

import type { ToolApprovalGrant, ToolApprovalGrantSetRequest } from "../types";
import {
  useRevokeToolApprovalGrant,
  useSetToolApprovalGrant,
} from "./use-tool-approval-grant-actions";
import { useToolApprovalGrants } from "./use-tool-approval-grants";

const TERMINAL_BROADER_DECISION_ERROR =
  "Terminal run and typing decisions come from a prompt, not a broader remembered allow.";

export type ToolApprovalGrantsState = "loading" | "error" | "empty" | "ready";

export interface ToolApprovalGrantSetDraft {
  toolId: string;
  decision: "" | "allow" | "reject";
  scope: "" | "agent" | "tool";
  agentName: string;
}

export interface ToolApprovalGrantsSetViewModel {
  draft: ToolApprovalGrantSetDraft;
  isOpen: boolean;
  isPending: boolean;
  canSubmit: boolean;
  error: string | null;
  open: () => void;
  close: () => void;
  change: (draft: ToolApprovalGrantSetDraft) => void;
  submit: () => void;
}

export interface ToolApprovalGrantsRevokeViewModel {
  /** The grant currently pending confirmation, or `null` when the dialog is closed. */
  target: ToolApprovalGrant | null;
  isOpen: boolean;
  isPending: boolean;
  /** Revoke failure message, kept visible in the dialog so the operator can retry. */
  error: string | null;
  open: (grant: ToolApprovalGrant) => void;
  close: () => void;
  confirm: () => void;
}

export interface ToolApprovalGrantsPanelViewModel {
  hasWorkspace: boolean;
  state: ToolApprovalGrantsState;
  /** Flattened grants in daemon order (never re-sorted client-side). */
  grants: ToolApprovalGrant[];
  /** Exact daemon-owned total for the active workspace. */
  total: number;
  error: Error | null;
  onRetry: () => void;
  set: ToolApprovalGrantsSetViewModel;
  revoke: ToolApprovalGrantsRevokeViewModel;
}

const EMPTY_SET_DRAFT: ToolApprovalGrantSetDraft = {
  toolId: "",
  decision: "",
  scope: "",
  agentName: "",
};

function setRequestFromDraft(draft: ToolApprovalGrantSetDraft): ToolApprovalGrantSetRequest | null {
  const toolId = draft.toolId.trim();
  const agentName = draft.agentName.trim();
  if (toolId === "" || draft.decision === "" || draft.scope === "") return null;
  if (isTerminalBroaderDecisionForbidden(toolId)) return null;
  if (draft.scope === "agent" && agentName === "") return null;
  return {
    tool_id: toolId,
    decision: draft.decision,
    scope: draft.scope,
    ...(draft.scope === "agent" ? { agent_name: agentName } : {}),
  };
}

/**
 * View model for the Permissions "Remembered decisions" section: scopes the read to the
 * runtime workspace, derives local loading/error/empty/ready states, and owns the revoke
 * confirmation lifecycle (target, pending, error) so the component stays presentational.
 */
export function useToolApprovalGrantsPanel(): ToolApprovalGrantsPanelViewModel {
  const { runtimeWorkspaceId, hasHydrated, isLoading: workspacesLoading } = useActiveWorkspace();
  const workspaceId = runtimeWorkspaceId ?? "";
  const hasWorkspace = workspaceId !== "";

  const { data, error: queryErrorRaw, isLoading, refetch } = useToolApprovalGrants(workspaceId);
  const grants = data?.grants ?? [];
  const total = data?.total ?? 0;
  const queryError = queryErrorRaw instanceof Error ? queryErrorRaw : null;

  const resolvingWorkspace = !hasHydrated || workspacesLoading;
  const state: ToolApprovalGrantsState =
    resolvingWorkspace || (hasWorkspace && isLoading)
      ? "loading"
      : queryError
        ? "error"
        : grants.length === 0
          ? "empty"
          : "ready";

  function onRetry() {
    void refetch();
  }

  const set = useToolApprovalGrantSetModel({ hasWorkspace, workspaceId });
  const revoke = useToolApprovalGrantRevokeModel();

  return {
    hasWorkspace,
    state,
    grants,
    total,
    error: queryError,
    onRetry,
    set,
    revoke,
  };
}

function useToolApprovalGrantRevokeModel(): ToolApprovalGrantsRevokeViewModel {
  const mutation = useRevokeToolApprovalGrant();
  const [target, setTarget] = useState<ToolApprovalGrant | null>(null);

  return {
    target,
    isOpen: target !== null,
    isPending: mutation.isPending,
    error: mutation.error instanceof Error ? mutation.error.message : null,
    open: grant => {
      mutation.reset();
      setTarget(grant);
    },
    close: () => {
      if (mutation.isPending) return;
      mutation.reset();
      setTarget(null);
    },
    confirm: () => {
      if (!target || mutation.isPending) return;
      // Bind the revoke to the grant's own workspace, not the workspace active at
      // confirmation time — a workspace switch between opening and confirming must
      // not revoke against (or invalidate) the wrong workspace's list.
      mutation.mutate(
        { id: target.id, workspaceId: target.workspace_id },
        { onSuccess: () => setTarget(null) }
      );
    },
  };
}

function useToolApprovalGrantSetModel({
  hasWorkspace,
  workspaceId,
}: {
  hasWorkspace: boolean;
  workspaceId: string;
}): ToolApprovalGrantsSetViewModel {
  const mutation = useSetToolApprovalGrant();
  const [draft, setDraft] = useState<ToolApprovalGrantSetDraft>(EMPTY_SET_DRAFT);
  const [boundWorkspaceId, setBoundWorkspaceId] = useState("");
  const [isOpen, setIsOpen] = useState(false);
  const request = setRequestFromDraft(draft);

  return {
    draft,
    isOpen,
    isPending: mutation.isPending,
    canSubmit: request !== null,
    error:
      mutation.error instanceof Error
        ? mutation.error.message
        : isTerminalBroaderDecisionForbidden(draft.toolId.trim())
          ? TERMINAL_BROADER_DECISION_ERROR
          : null,
    open: () => {
      if (!hasWorkspace) return;
      mutation.reset();
      setDraft(EMPTY_SET_DRAFT);
      setBoundWorkspaceId(workspaceId);
      setIsOpen(true);
    },
    close: () => {
      if (mutation.isPending) return;
      mutation.reset();
      setIsOpen(false);
    },
    change: next => setDraft(next.scope === "tool" ? { ...next, agentName: "" } : next),
    submit: () => {
      if (request === null || mutation.isPending || boundWorkspaceId === "") return;
      mutation.mutate(
        { workspaceId: boundWorkspaceId, request },
        { onSuccess: () => setIsOpen(false) }
      );
    },
  };
}
