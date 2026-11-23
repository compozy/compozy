import { useState } from "react";
import {
  Bell,
  ChevronDown,
  MessagesSquare,
  Plus,
  RefreshCw,
  Smartphone,
  Terminal,
  WifiOff,
  X,
} from "@compozy/ui/icons";

import { Button, cn, Logo, PillCount, StatusDot, toast } from "@compozy/ui";

import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import { PhoneComputerMenu } from "@/systems/computers/components/phone-computer-menu";
import type {
  ComputerFixture,
  HostedPeerFixture,
  PendingApprovalFixture,
  RemoteScope,
  UnifiedSessionFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import {
  PHONE_APPROVALS_ON_STUDIO,
  PHONE_FULL_SCOPES,
  PHONE_PEERS_MULTI,
  PHONE_STUDIO_SESSIONS,
} from "@/systems/computers/mocks/remote-computers-fixtures-phone";

import { PhoneApprovalsPane, PhoneSessionList, PhoneTerminalPane } from "./phone-panes";
import { PhoneSessionView } from "./phone-session-view";

export type PhoneTabId = "sessions" | "terminal" | "approvals";

export interface PhoneAppProps {
  computer: ComputerFixture;
  peers?: readonly HostedPeerFixture[];
  sessions?: readonly UnifiedSessionFixture[];
  approvals?: readonly PendingApprovalFixture[];
  scopes?: readonly RemoteScope[];
  initialTab?: PhoneTabId;
  initialOpenSessionId?: string | null;
  initialComputerMenuOpen?: boolean;
  offline?: boolean;
  showInstallHint?: boolean;
}

/**
 * Signed-in CompozyOS browser SPA served directly by one computer's hosted route
 * (`https://studio-1c9d.compozy.run`).
 *
 * Phone form only (<1024 px): compact header with the computer picker, bottom tab bar
 * (Sessions, Terminal, Approvals) and a bottom-sheet computer menu. Wider browsers get the
 * CompozyOS web app shell (`HostedWebApp`).
 */
export function PhoneApp({
  computer,
  peers = PHONE_PEERS_MULTI,
  sessions = PHONE_STUDIO_SESSIONS,
  approvals: initialApprovals = PHONE_APPROVALS_ON_STUDIO,
  scopes = PHONE_FULL_SCOPES,
  initialTab = "sessions",
  initialOpenSessionId = null,
  initialComputerMenuOpen = false,
  offline: initialOffline = false,
  showInstallHint: initialShowInstallHint = true,
}: PhoneAppProps) {
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

  const renderHeader = () => (
    <header className="flex h-12 shrink-0 items-center justify-between gap-2 border-b border-line-soft px-3">
      <div className="flex min-w-0 items-center gap-2">
        <Logo className="size-5" decorative variant="symbol" />
        <button
          aria-expanded={computerMenuOpen}
          aria-haspopup="dialog"
          className="relative inline-flex h-7 max-w-[200px] items-center gap-1.5 rounded-pill border border-line bg-surface-2 px-2.5 text-small-body font-medium text-fg transition-colors after:absolute after:-inset-x-1 after:-inset-y-1.5 after:content-[''] hover:bg-selected focus-visible:shadow-focus-ring focus-visible:outline-none"
          onClick={() => setComputerMenuOpen(true)}
          type="button"
        >
          <ComputerPlatformIcon
            className="size-3.5 bg-transparent"
            iconKind={computer.iconKind}
            platform={computer.platform}
            size="sm"
          />
          <span className="truncate">{computer.name}</span>
          <StatusDot size="sm" tone={offline ? "warning" : "success"} />
          <ChevronDown aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
        </button>
      </div>

      <Button
        aria-label={`New session on ${computer.name}`}
        disabled={offline}
        onClick={() => toast.message(`New session on ${computer.name}`)}
        size="icon-sm"
        variant="secondary"
      >
        <Plus className="size-4" />
      </Button>
    </header>
  );

  const renderOfflineStrip = () =>
    offline ? (
      <div
        className="flex shrink-0 items-center justify-between gap-2 border-b border-line-soft bg-warning-tint px-3.5 py-2"
        data-slot="phone-offline-strip"
        role="status"
      >
        <div className="flex min-w-0 items-center gap-2 text-eyebrow text-fg">
          <WifiOff aria-hidden="true" className="size-3.5 shrink-0 text-warning" />
          <span className="truncate">{computer.name} is offline. Reconnecting…</span>
        </div>
        <Button className="shrink-0" onClick={handleRetry} size="xs" variant="secondary">
          <RefreshCw className="size-3" />
          Retry now
        </Button>
      </div>
    ) : null;

  const renderSessionList = () => (
    <PhoneSessionList
      filteredSessions={filteredSessions}
      offline={offline}
      onOpenSession={sessionId => {
        setOpenSessionId(sessionId);
        setActiveTab("sessions");
      }}
      onSelectProject={setProjectFilter}
      projectFilter={projectFilter}
      projects={projects}
      sessions={sessions}
    />
  );

  if (openSession) {
    return (
      <div className="relative flex min-h-0 flex-1 flex-col" data-layout="phone">
        <PhoneSessionView
          canOperateTerminal={canOperateTerminal}
          computerName={computer.name}
          offline={offline}
          onBack={() => setOpenSessionId(null)}
          onOpenTerminal={
            canOperateTerminal
              ? () => {
                  setOpenSessionId(null);
                  setActiveTab("terminal");
                }
              : undefined
          }
          onRetryConnection={handleRetry}
          session={openSession}
        />
      </div>
    );
  }

  return (
    <div
      className="relative flex min-h-0 flex-1 flex-col bg-canvas"
      data-layout="phone"
      data-slot="phone-app"
    >
      {renderHeader()}
      {renderOfflineStrip()}

      {installHintVisible && !offline && activeTab === "sessions" ? (
        <div className="flex shrink-0 items-center justify-between gap-2 border-b border-line-soft bg-sunken px-3.5 py-2 text-eyebrow text-muted-foreground">
          <div className="flex min-w-0 items-center gap-2">
            <Smartphone aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
            <span className="truncate">Add to Home Screen for full-screen access</span>
          </div>
          <Button
            aria-label="Dismiss install hint"
            onClick={() => setInstallHintVisible(false)}
            size="icon-xs"
            variant="quiet"
          >
            <X className="size-3" />
          </Button>
        </div>
      ) : null}

      {activeTab === "sessions" ? renderSessionList() : null}
      {activeTab === "terminal" && canOperateTerminal ? (
        <PhoneTerminalPane computer={computer} offline={offline} />
      ) : null}
      {activeTab === "approvals" ? (
        <PhoneApprovalsPane
          approvals={approvals}
          computer={computer}
          onDecide={handleDecideApproval}
        />
      ) : null}

      <nav
        aria-label="Phone navigation"
        className="flex shrink-0 items-center justify-around border-t border-line-soft bg-sunken px-2 py-1.5"
        data-slot="phone-bottom-dock"
      >
        <button
          aria-current={activeTab === "sessions" ? "page" : undefined}
          className={cn(
            "flex min-h-11 flex-1 flex-col items-center justify-center gap-0.5 rounded-lg px-2 py-1 text-micro transition-colors focus-visible:shadow-focus-ring focus-visible:outline-none",
            activeTab === "sessions" ? "bg-selected font-medium text-fg-strong" : "text-subtle"
          )}
          onClick={() => setActiveTab("sessions")}
          type="button"
        >
          <MessagesSquare className="size-4" />
          <span>Sessions</span>
        </button>

        {canOperateTerminal ? (
          <button
            aria-current={activeTab === "terminal" ? "page" : undefined}
            className={cn(
              "flex min-h-11 flex-1 flex-col items-center justify-center gap-0.5 rounded-lg px-2 py-1 text-micro transition-colors focus-visible:shadow-focus-ring focus-visible:outline-none",
              activeTab === "terminal" ? "bg-selected font-medium text-fg-strong" : "text-subtle"
            )}
            onClick={() => setActiveTab("terminal")}
            type="button"
          >
            <Terminal className="size-4" />
            <span>Terminal</span>
          </button>
        ) : null}

        <button
          aria-current={activeTab === "approvals" ? "page" : undefined}
          className={cn(
            "relative flex min-h-11 flex-1 flex-col items-center justify-center gap-0.5 rounded-lg px-2 py-1 text-micro transition-colors focus-visible:shadow-focus-ring focus-visible:outline-none",
            activeTab === "approvals" ? "bg-selected font-medium text-fg-strong" : "text-subtle"
          )}
          onClick={() => setActiveTab("approvals")}
          type="button"
        >
          <span className="relative inline-flex">
            <Bell className="size-4" />
            {pendingApprovalCount > 0 ? (
              <PillCount className="absolute -top-1.5 -right-2.5" count={pendingApprovalCount} />
            ) : null}
          </span>
          <span>Approvals</span>
        </button>
      </nav>

      <PhoneComputerMenu
        currentComputerName={computer.name}
        onClose={() => setComputerMenuOpen(false)}
        onSelectPeer={peer => {
          setComputerMenuOpen(false);
          toast.message(`Opening https://${peer.hostname}`);
        }}
        onSignOut={handleSignOut}
        open={computerMenuOpen}
        peers={peers}
        presentation="sheet"
      />
    </div>
  );
}
