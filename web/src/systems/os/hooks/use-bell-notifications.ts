import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { useDocumentVisible } from "@/hooks/use-document-visible";
import { dashboardKeys } from "@/systems/dashboard";
import {
  acknowledgeAttentionNotifications,
  attentionNotificationsOptions,
  notificationKeys,
  type AttentionNotification,
} from "@/systems/notifications";
import { useProfileReadScope } from "@/systems/profiles";
import { toSessionBadge } from "@/systems/session";
import type { OsAttentionRow, OsAttentionSections } from "../lib/attention-model";

export function useBellNotifications(mutedWorkspaceIds: ReadonlySet<string>) {
  const { destination } = useProfileReadScope();
  const visible = useDocumentVisible();
  const queryClient = useQueryClient();
  const query = useQuery({
    ...attentionNotificationsOptions(destination),
    refetchInterval: visible ? 5_000 : false,
  });
  const mutation = useMutation({
    mutationFn: ({ snapshot, id, profile }: { snapshot: string; id?: string; profile: string }) =>
      acknowledgeAttentionNotifications(
        { surface: "bell", profile, receipt_profile: profile },
        { snapshot, id }
      ),
    onSettled: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: notificationKeys.attentionRoot() }),
        queryClient.invalidateQueries({ queryKey: dashboardKeys.overviewRoot() }),
      ]),
  });
  const sections: OsAttentionSections = { needsYou: [], finished: [] };
  const stale = query.isError || query.data === undefined;
  for (const item of query.data?.items ?? []) {
    const row = notificationRow(item, mutedWorkspaceIds, stale);
    if (item.finished && row.kind === "session") sections.finished.push(row);
    else sections.needsYou.push(row);
  }
  return {
    sections,
    count: stale ? 0 : (query.data?.needs_you ?? 0),
    total: query.data?.total ?? 0,
    snapshot: query.data?.snapshot,
    stale,
    loading: query.isLoading,
    pending: mutation.isPending,
    error: mutation.error?.message ?? null,
    acknowledge: (row?: OsAttentionRow) => {
      const snapshot = query.data?.snapshot;
      if (!snapshot || stale || mutation.isPending || (row && !row.notificationId)) return;
      mutation.mutate({ snapshot, id: row?.notificationId, profile: destination });
    },
  };
}

function notificationRow(
  item: AttentionNotification,
  muted: ReadonlySet<string>,
  stale: boolean
): OsAttentionRow {
  const common = {
    id: item.source_id,
    notificationId: item.id,
    title: item.title,
    workspaceId: item.workspace_id,
    workspaceLabel: item.workspace_label,
  };
  switch (item.kind) {
    case "session":
      return {
        ...common,
        kind: "session",
        badge: toSessionBadge(item.badge),
        agentName: item.agent_name ?? "",
        reason: item.detail,
        changedAt: item.occurred_at,
        muted: muted.has(item.workspace_id),
        stale,
      };
    case "loop-request":
      return {
        ...common,
        kind: "loop-request",
        runId: item.run_id ?? "",
        nodeId: item.node_id ?? "",
        itemIndex: item.item_index,
        loopName: item.loop_name ?? "",
        openedAt: item.occurred_at,
        requestKind: item.request_kind === "review" ? "review" : "ask",
        stale,
      };
    case "loop-node":
      return {
        ...common,
        kind: "loop-node",
        runId: item.run_id,
        state: item.detail === "waiting" ? "waiting" : "attention",
      };
    case "terminal-input":
      return {
        ...common,
        kind: "terminal-input",
        agentName: item.agent_name ?? "",
        terminalId: item.terminal_id ?? "",
        reason: item.detail,
        requestedAt: item.occurred_at,
        redacted: item.redacted,
        stale,
      };
    default:
      return { ...common, kind: "task", reason: item.detail };
  }
}
