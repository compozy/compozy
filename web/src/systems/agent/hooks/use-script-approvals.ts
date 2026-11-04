import { useNavigate } from "@tanstack/react-router";

import {
  scriptApprovalsFromInstances,
  type ScriptApprovalItem,
} from "../lib/agent-script-approvals";
import { useApproveWaitingScript } from "./use-heartbeat-sources";
import { useAgentInstancesState } from "./use-agent-instances";

/**
 * The scripts waiting for approval across every always-on placement the operator can see, read
 * from the same live instance list as the Agents window, with approve and review actions.
 */
export function useScriptApprovals() {
  const { instances } = useAgentInstancesState();
  const approve = useApproveWaitingScript();
  const navigate = useNavigate();
  return {
    items: scriptApprovalsFromInstances(instances),
    /** The key of the approval on its way, if any. */
    approving: approve.isPending ? (approve.variables?.key ?? null) : null,
    approve: (item: ScriptApprovalItem) => approve.mutate(item),
    review: (item: ScriptApprovalItem) =>
      void navigate({
        to: "/agents/$name/$page",
        params: { name: item.agentName, page: "check-ins" },
        search: { workspace: item.workspace, ...(item.profile ? { profile: item.profile } : {}) },
      }),
  };
}

export type UseScriptApprovals = ReturnType<typeof useScriptApprovals>;
