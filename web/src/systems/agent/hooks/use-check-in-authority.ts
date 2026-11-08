import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast } from "@compozy/ui";

import { approveAgentHeartbeatAuthority } from "../adapters/agent-heartbeat-api";
import { isCheckInWritableTool } from "../lib/agent-check-in-authority";
import { agentKeys } from "../lib/query-keys";
import { agentToolsOptions } from "../lib/query-options";
import type { AgentHeartbeatPayload } from "../types";

export type HeartbeatAuthority = AgentHeartbeatPayload["authority"];

interface PendingConfirm {
  authority: HeartbeatAuthority;
  settle: (approve: boolean) => void;
}

/**
 * Check-in authority on the Heartbeat page: the write tools to pick from, Approve for a block
 * that waits, and the confirmation a save asks for when it changes the block. A save never
 * approves on its own; the digest goes out only after the operator saw the block and confirmed.
 */
export function useCheckInAuthority(
  name: string,
  workspace: string | null,
  profile: string,
  current: HeartbeatAuthority | undefined,
  enabled: boolean
) {
  const queryClient = useQueryClient();
  const tools = useQuery({ ...agentToolsOptions(name, workspace, profile), enabled });
  const [pending, setPending] = useState<PendingConfirm | null>(null);
  const approve = useMutation({
    mutationFn: (sha256: string) =>
      approveAgentHeartbeatAuthority(name, sha256, workspace, profile),
    onSuccess: () => toast.message("Approved. Check-ins can use these changes now."),
    onError: error => toast.error(error instanceof Error ? error.message : "Couldn't approve."),
    onSettled: () => {
      void queryClient.invalidateQueries({
        queryKey: agentKeys.heartbeat(name, workspace, profile),
      });
      void queryClient.invalidateQueries({ queryKey: agentKeys.instances() });
    },
  });

  /**
   * Whether a save of a candidate with this authority carries approval: asked only when the block
   * changed from the saved one and isn't approved yet.
   */
  const confirmSave = (candidate: HeartbeatAuthority | undefined) => {
    if (!candidate || candidate.status !== "unapproved" || candidate.digest === current?.digest) {
      return Promise.resolve(false);
    }
    return new Promise<boolean>(resolve => {
      setPending({
        authority: candidate,
        settle: approved => {
          setPending(null);
          resolve(approved);
        },
      });
    });
  };

  return {
    current,
    writeTools: (tools.data ?? []).filter(tool => isCheckInWritableTool(tool)),
    toolsState: tools.isError
      ? ("error" as const)
      : tools.data
        ? ("ready" as const)
        : ("loading" as const),
    approve: () => {
      if (current?.digest) approve.mutate(current.digest);
    },
    approving: approve.isPending,
    confirmSave,
    /** The save waiting on the operator's answer, with the block it would approve. */
    pending,
  };
}

export type CheckInAuthorityModel = ReturnType<typeof useCheckInAuthority>;
