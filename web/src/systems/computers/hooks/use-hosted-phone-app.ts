import { useState } from "react";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import {
  approveSession,
  getSessionDisplayTitle,
  pendingInteractions,
  sessionKeys,
  sessionsListOptions,
  useSessionCatalogStreams,
  type SessionPayload,
} from "@/systems/session";
import { workspacesListOptions } from "@/systems/workspace";

import type { HostedPhoneApprovalItem } from "../components/hosted-phone-panes";
import { useHostedBrowserContext } from "./use-hosted-browser-context";

export type HostedPhoneTab = "sessions" | "approvals" | "terminal";

const INSTALL_HINT_DISMISSED_KEY = "compozy.hosted.phone.install-hint-dismissed";

function initialInstallHintVisible(): boolean {
  if (typeof window === "undefined") return false;
  try {
    if (window.matchMedia?.("(display-mode: standalone)").matches) return false;
    return window.localStorage?.getItem(INSTALL_HINT_DISMISSED_KEY) !== "1";
  } catch {
    return false;
  }
}

function collectPhoneApprovals(
  sessions: readonly SessionPayload[],
  workspaceNameById: ReadonlyMap<string, string>
): HostedPhoneApprovalItem[] {
  const items: HostedPhoneApprovalItem[] = [];
  for (const session of sessions) {
    const workspaceId = session.workspace_id?.trim() ?? "";
    const sessionTitle = getSessionDisplayTitle(session);
    const projectName = workspaceId
      ? (workspaceNameById.get(workspaceId) ?? session.workspace_path ?? "Project")
      : "User";
    for (const interaction of pendingInteractions(session)) {
      const requestId = interaction.provider_request_id?.trim() ?? "";
      const kind = interaction.kind === "permission" ? "permission" : "clarify";
      if (kind === "permission" && (!requestId || !workspaceId)) continue;
      const interactionTitle =
        interaction.title?.trim() || (kind === "permission" ? "Approval needed" : "Input needed");
      items.push({
        id: `${session.id}:${requestId || interactionTitle}`,
        requestId,
        turnId: interaction.turn_id ?? "",
        sessionId: session.id,
        workspaceId,
        sessionTitle,
        projectName,
        title: interactionTitle,
        command: interactionTitle,
        kind,
      });
    }
  }
  return items;
}

/**
 * Controller hook for the compact hosted phone layout (`< 1024px`, U8):
 * sessions list + live catalog stream, cross-session command approvals,
 * optional terminal tab gated by `terminal:operate`, and computer switcher sheet.
 */
export function useHostedPhoneApp() {
  const queryClient = useQueryClient();
  const hosted = useHostedBrowserContext();

  const [activeTab, setActiveTab] = useState<HostedPhoneTab>("sessions");
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null);
  const [computerMenuOpen, setComputerMenuOpen] = useState(false);
  const [installHintVisible, setInstallHintVisible] = useState(initialInstallHintVisible);
  const [pendingDecisionIds, setPendingDecisionIds] = useState<Set<string>>(() => new Set());

  const sessionsQuery = useInfiniteQuery(sessionsListOptions({ all_profiles: true }));
  const workspacesQuery = useQuery(workspacesListOptions(true));
  useSessionCatalogStreams({ enabled: true });

  const workspaces = workspacesQuery.data ?? [];
  const workspaceNameById = new Map<string, string>(
    workspaces.map(workspace => [workspace.id, workspace.name])
  );
  const sessions = sessionsQuery.data?.pages.flatMap(page => page.sessions) ?? [];
  const selectedSession = selectedSessionId
    ? (sessions.find(item => item.id === selectedSessionId) ?? null)
    : null;
  const approvals = collectPhoneApprovals(sessions, workspaceNameById);

  const canOperateTerminal = hosted?.canOperateTerminal ?? false;
  const resolvedTab: HostedPhoneTab =
    activeTab === "terminal" && !canOperateTerminal ? "sessions" : activeTab;

  const terminalWorkspaceId =
    selectedSession?.workspace_id?.trim() ||
    sessions.find(item => Boolean(item.workspace_id?.trim()))?.workspace_id?.trim() ||
    workspaces[0]?.id ||
    "";
  const terminalProjectLabel = terminalWorkspaceId
    ? workspaceNameById.get(terminalWorkspaceId)
    : undefined;

  const decideMutation = useMutation({
    mutationFn: async ({ item, approve }: { item: HostedPhoneApprovalItem; approve: boolean }) => {
      await approveSession(item.workspaceId, item.sessionId, {
        request_id: item.requestId,
        turn_id: item.turnId,
        decision: approve ? "allow-once" : "reject-once",
      });
    },
    onMutate: ({ item }) => {
      setPendingDecisionIds(prev => {
        const next = new Set(prev);
        next.add(item.id);
        return next;
      });
    },
    onSettled: (_data, _error, { item }) => {
      setPendingDecisionIds(prev => {
        if (!prev.has(item.id)) return prev;
        const next = new Set(prev);
        next.delete(item.id);
        return next;
      });
    },
    onSuccess: (_data, { approve }) => {
      void queryClient.invalidateQueries({ queryKey: sessionKeys.all });
      toast.success(approve ? "Approved command." : "Denied command.");
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Couldn't apply decision.");
    },
  });

  const dismissInstallHint = () => {
    setInstallHintVisible(false);
    try {
      window.localStorage?.setItem(INSTALL_HINT_DISMISSED_KEY, "1");
    } catch {
      // Ignore private-mode storage errors.
    }
  };

  return {
    hosted,
    activeTab: resolvedTab,
    setActiveTab,
    selectedSession,
    selectedSessionProjectName: selectedSession?.workspace_id
      ? (workspaceNameById.get(selectedSession.workspace_id) ??
        selectedSession.workspace_path ??
        "Project")
      : "User",
    openSession: (session: SessionPayload) => setSelectedSessionId(session.id),
    openSessionById: (sessionId: string) => {
      setSelectedSessionId(sessionId);
      setActiveTab("sessions");
    },
    closeSession: () => setSelectedSessionId(null),
    openTerminalFromSession: () => {
      if (!canOperateTerminal) return;
      setSelectedSessionId(null);
      setActiveTab("terminal");
    },
    sessions,
    sessionsLoading: sessionsQuery.isLoading,
    sessionsError: sessionsQuery.error instanceof Error ? sessionsQuery.error : null,
    retrySessions: () => void sessionsQuery.refetch(),
    workspaceNameById,
    approvals,
    pendingDecisionIds,
    approveItem: (item: HostedPhoneApprovalItem) => decideMutation.mutate({ item, approve: true }),
    denyItem: (item: HostedPhoneApprovalItem) => decideMutation.mutate({ item, approve: false }),
    canOperateTerminal,
    terminalWorkspaceId,
    terminalProjectLabel,
    computerMenuOpen,
    setComputerMenuOpen,
    installHintVisible,
    dismissInstallHint,
  };
}
