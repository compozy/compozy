import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { useDocumentVisible } from "@/hooks/use-document-visible";
import { useProfilePerProfileScope } from "@/systems/profiles";

import {
  scriptApprovalsFromInstances,
  type ScriptApprovalItem,
} from "../lib/agent-script-approvals";
import { useApproveWaitingScript } from "./use-heartbeat-sources";
import { agentInstancesOptions } from "../lib/query-options";

const SCRIPT_APPROVAL_REREAD_MS = 30_000;

/**
 * The scripts waiting for approval across every always-on placement the operator can see, read
 * from the same live instance list as the Agents window, with approve and review actions.
 */
export function useScriptApprovals() {
  const { key, names } = useProfilePerProfileScope();
  const visible = useDocumentVisible();
  // A script edit changes no instance and fires no event, so the bell rereads while visible.
  const query = useQuery({
    ...agentInstancesOptions(key, names),
    refetchInterval: visible ? SCRIPT_APPROVAL_REREAD_MS : false,
  });
  const approve = useApproveWaitingScript();
  const navigate = useNavigate();
  return {
    items: scriptApprovalsFromInstances(query.data?.instances ?? []),
    /** Rereads now, as the bell opens. */
    refresh: () => void query.refetch(),
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
