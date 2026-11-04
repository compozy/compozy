import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";

import { useDocumentVisible } from "@/hooks/use-document-visible";
import { useProfilePerProfileScope } from "@/systems/profiles";

import {
  scriptApprovalsFromInstances,
  type ScriptApprovalItem,
} from "../lib/agent-script-approvals";
import { useApproveWaitingScript } from "./use-heartbeat-sources";
import { agentInstancesOptions } from "../lib/query-options";

/** While the bell is open its rows are on screen; closed, only its count is. */
const OPEN_REREAD_MS = 30_000;
const CLOSED_REREAD_MS = 5 * 60_000;

/**
 * The scripts waiting for approval across every always-on placement the operator can see, read
 * from the same live instance list as the Agents window, with approve and review actions.
 */
export function useScriptApprovals({ open = false }: { open?: boolean } = {}) {
  const { key, names } = useProfilePerProfileScope();
  const visible = useDocumentVisible();
  // A script edit changes no instance and fires no event, so the bell rereads while visible:
  // often while open, rarely while closed, and once more as it opens (`refresh`).
  const query = useQuery({
    ...agentInstancesOptions(key, names),
    refetchInterval: visible ? (open ? OPEN_REREAD_MS : CLOSED_REREAD_MS) : false,
  });
  const { refetch } = query;
  // Every way the bell opens (click, toast, shortcut) rereads once, not just the trigger's own.
  useEffect(() => {
    if (open) void refetch();
  }, [open, refetch]);
  const approve = useApproveWaitingScript();
  const navigate = useNavigate();
  return {
    items: scriptApprovalsFromInstances(query.data?.instances ?? []),
    /** The list never loaded: the bell can't say whether anything waits. */
    unavailable: query.isError && query.data === undefined,
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
