import { useState } from "react";

import { useActiveWorkspace } from "@/systems/workspace";

import type { ToolApprovalGrant, ToolApprovalGrantSetRequest } from "../types";
import {
  useRevokeToolApprovalGrant,
  useSetToolApprovalGrant,
} from "./use-tool-approval-grant-actions";
import { useToolApprovalGrants } from "./use-tool-approval-grants";

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
 * active workspace, derives local loading/error/empty/ready states, and owns the revoke
 * confirmation lifecycle (target, pending, error) so the component stays presentational.
 */
export function useToolApprovalGrantsPanel(): ToolApprovalGrantsPanelViewModel {
  const { activeWorkspaceId, hasHydrated, isLoading: workspacesLoading } = useActiveWorkspace();
  const workspaceId = activeWorkspaceId ?? "";
  const hasWorkspace = workspaceId !== "";

  const { data, error: queryErrorRaw, isLoading, refetch } = useToolApprovalGrants(workspaceId);
  const {
    mutate: revokeMutate,
    reset: revokeReset,
    isPending: revokePending,
    error: revokeErrorRaw,
  } = useRevokeToolApprovalGrant();
  const {
    mutate: setMutate,
    reset: setReset,
    isPending: setPending,
    error: setErrorRaw,
  } = useSetToolApprovalGrant();
  const [target, setTarget] = useState<ToolApprovalGrant | null>(null);
  const [setDraft, setSetDraft] = useState<ToolApprovalGrantSetDraft>(EMPTY_SET_DRAFT);
  const [setWorkspaceId, setSetWorkspaceId] = useState("");
  const [isSetOpen, setIsSetOpen] = useState(false);

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

  function open(grant: ToolApprovalGrant) {
    revokeReset();
    setTarget(grant);
  }

  function close() {
    if (revokePending) return;
    revokeReset();
    setTarget(null);
  }

  function confirm() {
    if (!target || revokePending) return;
    // Bind the revoke to the grant's own workspace, not the workspace active at
    // confirmation time — a workspace switch between opening and confirming must
    // not revoke against (or invalidate) the wrong workspace's list.
    revokeMutate(
      { id: target.id, workspaceId: target.workspace_id },
      { onSuccess: () => setTarget(null) }
    );
  }

  function openSet() {
    if (!hasWorkspace) return;
    setReset();
    setSetDraft(EMPTY_SET_DRAFT);
    setSetWorkspaceId(workspaceId);
    setIsSetOpen(true);
  }

  function closeSet() {
    if (setPending) return;
    setReset();
    setIsSetOpen(false);
  }

  function changeSet(draft: ToolApprovalGrantSetDraft) {
    setSetDraft(draft.scope === "tool" ? { ...draft, agentName: "" } : draft);
  }

  const setRequest = setRequestFromDraft(setDraft);
  const canSubmitSet = setRequest !== null;

  function submitSet() {
    if (setRequest === null || setPending || setWorkspaceId === "") return;
    setMutate(
      {
        workspaceId: setWorkspaceId,
        request: setRequest,
      },
      { onSuccess: () => setIsSetOpen(false) }
    );
  }

  return {
    hasWorkspace,
    state,
    grants,
    total,
    error: queryError,
    onRetry,
    set: {
      draft: setDraft,
      isOpen: isSetOpen,
      isPending: setPending,
      canSubmit: canSubmitSet,
      error: setErrorRaw instanceof Error ? setErrorRaw.message : null,
      open: openSet,
      close: closeSet,
      change: changeSet,
      submit: submitSet,
    },
    revoke: {
      target,
      isOpen: target !== null,
      isPending: revokePending,
      error: revokeErrorRaw instanceof Error ? revokeErrorRaw.message : null,
      open,
      close,
      confirm,
    },
  };
}
