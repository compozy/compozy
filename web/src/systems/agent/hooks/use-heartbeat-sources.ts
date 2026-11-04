import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import {
  AgentHeartbeatScriptApprovalError,
  approveAgentHeartbeatScript,
} from "../adapters/agent-heartbeat-api";
import type { ScriptApprovalItem } from "../lib/agent-script-approvals";
import { agentKeys } from "../lib/query-keys";
import { agentHeartbeatSourcesOptions } from "../lib/query-options";

/** The catalog query itself, for callers that tell loading apart from a failed read. */
export function useHeartbeatSourceCatalogQuery(
  name: string,
  workspace: string | null,
  profile: string,
  enabled: boolean
) {
  return useQuery({
    ...agentHeartbeatSourcesOptions(name, workspace, profile),
    enabled: enabled && name !== "",
  });
}

/**
 * The kinds and scripts an agent's check-ins can read, for titles and limits. Only read while
 * something needs it; `null` until it answers, so callers fall back to the declared names.
 */
export function useHeartbeatSourceCatalog(
  name: string,
  workspace: string | null,
  profile: string,
  enabled: boolean
) {
  return useHeartbeatSourceCatalogQuery(name, workspace, profile, enabled).data ?? null;
}

function approvalProblem(error: unknown) {
  if (error instanceof AgentHeartbeatScriptApprovalError) {
    if (error.code === "source_script_changed") {
      return "This script changed since you looked. Check the new version, then approve it again.";
    }
    if (error.code === "source_approval_operator_only") {
      return "Only you can approve scripts. Agents can't approve them.";
    }
    if (error.status === 403) return "Scripts that come with an extension can't be approved.";
  }
  return error instanceof Error ? error.message : "Couldn't approve this script. Try again.";
}

/**
 * Approves a script's exact bytes by the hash the page showed. Either way the catalog is read
 * again, so a changed file shows its new hash before anyone approves it.
 */
export function useApproveHeartbeatScript(name: string, workspace: string | null, profile: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ script, sha256 }: { script: string; sha256: string }) =>
      approveAgentHeartbeatScript(name, script, sha256, workspace, undefined, profile),
    onSuccess: approval => {
      toast.message(`Approved ${approval.script}. It runs at the next check-in.`);
      void queryClient.invalidateQueries({ queryKey: agentKeys.instances() });
    },
    onError: error => toast.error(approvalProblem(error)),
    onSettled: () =>
      queryClient.invalidateQueries({
        queryKey: agentKeys.heartbeatSources(name, workspace, profile),
      }),
  });
}

/**
 * Approves a waiting script from outside its Heartbeat page (the shell's bell) with the same
 * hash-checked call. Afterwards every view of that agent's scripts reads again: the always-on
 * list that feeds the bell, and any open Heartbeat catalog for the agent.
 */
export function useApproveWaitingScript() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (item: ScriptApprovalItem) =>
      approveAgentHeartbeatScript(
        item.agentName,
        item.script,
        item.sha256,
        item.workspace,
        undefined,
        item.profile
      ),
    onSuccess: approval => {
      toast.message(`Approved ${approval.script}. It runs at the next check-in.`);
    },
    onError: error => toast.error(approvalProblem(error)),
    onSettled: (_approval, _error, item) =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: agentKeys.instanceLists() }),
        queryClient.invalidateQueries({
          predicate: query =>
            query.queryKey.includes(item.agentName) && query.queryKey.at(-1) === "sources",
        }),
      ]),
  });
}
