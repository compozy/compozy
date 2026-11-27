import { useState } from "react";

import { toast } from "@compozy/ui";

import type {
  ComputerFixture,
  PendingApprovalFixture,
  RemoteScope,
  UnifiedSessionFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";

export type PhoneTabId = "sessions" | "terminal" | "approvals";

interface UsePhoneAppInput {
  computer: ComputerFixture;
  sessions: readonly UnifiedSessionFixture[];
  initialApprovals: readonly PendingApprovalFixture[];
  scopes: readonly RemoteScope[];
  initialTab: PhoneTabId;
  initialOpenSessionId: string | null;
  initialComputerMenuOpen: boolean;
  initialOffline: boolean;
  initialShowInstallHint: boolean;
}

export function usePhoneApp({
  computer,
  sessions,
  initialApprovals,
  scopes,
  initialTab,
  initialOpenSessionId,
  initialComputerMenuOpen,
  initialOffline,
  initialShowInstallHint,
}: UsePhoneAppInput) {
  const canOperateTerminal = scopes.includes("terminal:operate");

  const [activeTab, setActiveTab] = useState<PhoneTabId>(() =>
    initialTab === "terminal" && !canOperateTerminal ? "sessions" : initialTab
  );
  const [openSessionId, setOpenSessionId] = useState<string | null>(initialOpenSessionId);
  const [computerMenuOpen, setComputerMenuOpen] = useState(initialComputerMenuOpen);
  const [projectFilter, setProjectFilter] = useState<string>("all");
  const [offline, setOffline] = useState(initialOffline);
  const [installHintVisible, setInstallHintVisible] = useState(initialShowInstallHint);
  const [approvals, setApprovals] = useState<PendingApprovalFixture[]>([...initialApprovals]);

  const openSession = openSessionId
    ? (sessions.find(item => item.id === openSessionId) ?? null)
    : null;

  const projects = Array.from(new Set(sessions.map(item => item.project)));
  const filteredSessions =
    projectFilter === "all" ? sessions : sessions.filter(item => item.project === projectFilter);
  const pendingApprovalCount = approvals.filter(item => item.state === "pending").length;

  const handleRetry = () => {
    setOffline(false);
    toast.success(`Reconnected to ${computer.name}`);
  };

  const handleSignOut = () => {
    setComputerMenuOpen(false);
    toast.message(`Signed out on ${computer.name}. This device stays registered on your account.`);
  };

  const handleDecideApproval = (approvalId: string, decision: "approved" | "denied") => {
    setApprovals(current =>
      current.map(entry => (entry.id === approvalId ? { ...entry, state: decision } : entry))
    );
  };

  return {
    canOperateTerminal,
    activeTab,
    setActiveTab,
    openSession,
    setOpenSessionId,
    computerMenuOpen,
    setComputerMenuOpen,
    projectFilter,
    setProjectFilter,
    offline,
    installHintVisible,
    setInstallHintVisible,
    approvals,
    projects,
    filteredSessions,
    pendingApprovalCount,
    handleRetry,
    handleSignOut,
    handleDecideApproval,
  };
}
