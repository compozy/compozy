import { useRef, useState } from "react";
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

import { Button, cn, Empty, Logo, PillCount, StatusDot, toast } from "@compozy/ui";

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

import { useIsWideBrowser } from "./phone-frame";
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
 * Signed-in Compozy browser SPA served directly by one computer's hosted route
 * (`https://studio-1c9d.remote.compozy.com`).
 *
 * - Below 1024 px: single-column phone layout with compact header, bottom tab bar,
 *   and bottom-sheet computer menu.
 * - At 1024 px and wider: two-pane web app (360 px left column with computer picker,
 *   segmented nav, filters and session list; right pane with the open session or a
 *   calm "Pick a session" state) and top-left popover computer menu.
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
  const rootRef = useRef<HTMLDivElement | null>(null);
  const isWide = useIsWideBrowser(rootRef);
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
    toast(`Signed out on ${computer.name}. This device stays registered on your account.`);
  };

  const handleDecideApproval = (approvalId: string, decision: "approved" | "denied") => {
    setApprovals(current =>
      current.map(entry => (entry.id === approvalId ? { ...entry, state: decision } : entry))
    );
  };

  const renderHeader = () => (
    <header className="flex h-14 shrink-0 items-center justify-between gap-2 border-b border-line-soft px-3.5">
      <div className="flex min-w-0 items-center gap-2">
        <Logo className="size-5.5" decorative variant="symbol" />
        <button
          aria-expanded={computerMenuOpen}
          aria-haspopup="dialog"
          className="inline-flex min-h-9 max-w-[200px] items-center gap-1.5 rounded-pill border border-line bg-surface-2 px-2.5 py-1 text-small-body font-medium text-fg transition-colors hover:bg-selected focus-visible:shadow-focus-ring focus-visible:outline-none"
          onClick={() => setComputerMenuOpen(true)}
          type="button"
        >
          <ComputerPlatformIcon
            className="size-4 bg-transparent"
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
        onClick={() => toast(`New session on ${computer.name}`)}
        size="icon"
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
      activeTab={activeTab}
      filteredSessions={filteredSessions}
      isWide={isWide}
      offline={offline}
      onOpenSession={sessionId => {
        setOpenSessionId(sessionId);
        setActiveTab("sessions");
      }}
      onSelectProject={setProjectFilter}
      openSessionId={openSessionId}
      projectFilter={projectFilter}
      projects={projects}
      sessions={sessions}
    />
  );

  if (isWide) {
    return (
      <div
        className="relative flex min-h-0 flex-1 bg-canvas"
        data-layout="wide"
        data-slot="phone-app"
        ref={rootRef}
      >
        <aside className="flex w-[360px] shrink-0 flex-col border-r border-line bg-sunken">
          {renderHeader()}

          <div
            aria-label="Workspace views"
            className="flex shrink-0 items-center gap-1 border-b border-line-soft px-3 py-2"
            role="tablist"
          >
            <button
              aria-selected={activeTab === "sessions"}
              className={cn(
                "inline-flex h-8 flex-1 items-center justify-center gap-1.5 rounded-md px-2.5 text-small-body transition-colors focus-visible:shadow-focus-ring focus-visible:outline-none",
                activeTab === "sessions"
                  ? "bg-selected font-medium text-fg-strong"
                  : "text-subtle hover:bg-surface-2 hover:text-fg"
              )}
              onClick={() => setActiveTab("sessions")}
              role="tab"
              type="button"
            >
              <MessagesSquare className="size-3.5" />
              <span>Sessions</span>
            </button>

            {canOperateTerminal ? (
              <button
                aria-selected={activeTab === "terminal"}
                className={cn(
                  "inline-flex h-8 flex-1 items-center justify-center gap-1.5 rounded-md px-2.5 text-small-body transition-colors focus-visible:shadow-focus-ring focus-visible:outline-none",
                  activeTab === "terminal"
                    ? "bg-selected font-medium text-fg-strong"
                    : "text-subtle hover:bg-surface-2 hover:text-fg"
                )}
                onClick={() => setActiveTab("terminal")}
                role="tab"
                type="button"
              >
                <Terminal className="size-3.5" />
                <span>Terminal</span>
              </button>
            ) : null}

            <button
              aria-selected={activeTab === "approvals"}
              className={cn(
                "inline-flex h-8 flex-1 items-center justify-center gap-1.5 rounded-md px-2.5 text-small-body transition-colors focus-visible:shadow-focus-ring focus-visible:outline-none",
                activeTab === "approvals"
                  ? "bg-selected font-medium text-fg-strong"
                  : "text-subtle hover:bg-surface-2 hover:text-fg"
              )}
              onClick={() => setActiveTab("approvals")}
              role="tab"
              type="button"
            >
              <Bell className="size-3.5" />
              <span>Approvals</span>
              {pendingApprovalCount > 0 ? <PillCount count={pendingApprovalCount} /> : null}
            </button>
          </div>

          {!openSession ? renderOfflineStrip() : null}
          {renderSessionList()}
        </aside>

        <main className="flex min-w-0 flex-1 flex-col overflow-hidden bg-canvas">
          {activeTab === "terminal" && canOperateTerminal ? (
            <PhoneTerminalPane computer={computer} isWide offline={offline} />
          ) : activeTab === "approvals" ? (
            <PhoneApprovalsPane
              approvals={approvals}
              computer={computer}
              isWide
              onDecide={handleDecideApproval}
            />
          ) : openSession ? (
            <PhoneSessionView
              canOperateTerminal={canOperateTerminal}
              computerName={computer.name}
              offline={offline}
              onOpenTerminal={canOperateTerminal ? () => setActiveTab("terminal") : undefined}
              onRetryConnection={handleRetry}
              session={openSession}
              wide
            />
          ) : (
            <div className="flex flex-1 items-center justify-center p-8">
              <Empty
                action={
                  <Button
                    disabled={offline}
                    onClick={() => toast(`New session on ${computer.name}`)}
                    size="default"
                    variant="secondary"
                  >
                    <Plus className="size-4" />
                    New session
                  </Button>
                }
                description={`Choose a session on ${computer.name} from the list, or start a new one.`}
                icon={MessagesSquare}
                title="Pick a session"
              />
            </div>
          )}
        </main>

        <PhoneComputerMenu
          currentComputerName={computer.name}
          onClose={() => setComputerMenuOpen(false)}
          onSelectPeer={peer => {
            setComputerMenuOpen(false);
            toast(`Opening https://${peer.hostname}`);
          }}
          onSignOut={handleSignOut}
          open={computerMenuOpen}
          peers={peers}
          presentation="popover"
        />
      </div>
    );
  }

  if (openSession) {
    return (
      <div className="relative flex min-h-0 flex-1 flex-col" data-layout="phone" ref={rootRef}>
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
      ref={rootRef}
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
        <PhoneTerminalPane computer={computer} isWide={false} offline={offline} />
      ) : null}
      {activeTab === "approvals" ? (
        <PhoneApprovalsPane
          approvals={approvals}
          computer={computer}
          isWide={false}
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
          toast(`Opening https://${peer.hostname}`);
        }}
        onSignOut={handleSignOut}
        open={computerMenuOpen}
        peers={peers}
        presentation="sheet"
      />
    </div>
  );
}
