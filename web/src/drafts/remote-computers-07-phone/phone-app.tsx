import {
  Bell,
  ChevronDown,
  MessagesSquare,
  Plus,
  Smartphone,
  Terminal,
  X,
} from "@compozy/ui/icons";

import { useState } from "react";

import {
  Button,
  cn,
  Logo,
  OverlayContainerContext,
  PillCount,
  StatusDot,
  toast,
} from "@compozy/ui";

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

import { usePhoneApp, type PhoneTabId } from "./hooks/use-phone-app";
import {
  PhoneApprovalsPane,
  PhoneOfflineStrip,
  PhoneSessionList,
  PhoneTerminalPane,
} from "./phone-panes";
import { PhoneSessionView } from "./phone-session-view";

export type { PhoneTabId };

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
  const app = usePhoneApp({
    computer,
    sessions,
    initialApprovals,
    scopes,
    initialTab,
    initialOpenSessionId,
    initialComputerMenuOpen,
    initialOffline,
    initialShowInstallHint,
  });

  const [overlayHost, setOverlayHost] = useState<HTMLDivElement | null>(null);

  if (app.openSession) {
    return (
      <div className="relative flex min-h-0 flex-1 flex-col" data-layout="phone">
        <PhoneSessionView
          canOperateTerminal={app.canOperateTerminal}
          computerName={computer.name}
          offline={app.offline}
          onBack={() => app.setOpenSessionId(null)}
          onOpenTerminal={
            app.canOperateTerminal
              ? () => {
                  app.setOpenSessionId(null);
                  app.setActiveTab("terminal");
                }
              : undefined
          }
          onRetryConnection={app.handleRetry}
          session={app.openSession}
        />
      </div>
    );
  }

  return (
    <div
      className="relative flex min-h-0 flex-1 transform-gpu flex-col bg-canvas"
      data-layout="phone"
      data-slot="phone-app"
      ref={setOverlayHost}
    >
      <header className="flex h-12 shrink-0 items-center justify-between gap-2 border-b border-line-soft px-3">
        <div className="flex min-w-0 items-center gap-2">
          <Logo className="size-5" decorative variant="symbol" />
          <button
            aria-expanded={app.computerMenuOpen}
            aria-haspopup="dialog"
            className="relative inline-flex h-7 max-w-[200px] items-center gap-1.5 rounded-pill border border-line bg-surface-2 px-2.5 text-small-body font-medium text-fg transition-colors after:absolute after:-inset-x-1 after:-inset-y-1.5 after:content-[''] hover:bg-selected focus-visible:shadow-focus-ring focus-visible:outline-none"
            onClick={() => app.setComputerMenuOpen(true)}
            type="button"
          >
            <ComputerPlatformIcon
              className="size-3.5 bg-transparent"
              iconKind={computer.iconKind}
              platform={computer.platform}
              size="sm"
            />
            <span className="truncate">{computer.name}</span>
            <StatusDot size="sm" tone={app.offline ? "warning" : "success"} />
            <ChevronDown aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
          </button>
        </div>

        <Button
          aria-label={`New session on ${computer.name}`}
          disabled={app.offline}
          onClick={() => toast.message(`New session on ${computer.name}`)}
          size="icon-sm"
          variant="secondary"
        >
          <Plus className="size-4" />
        </Button>
      </header>

      {app.offline && <PhoneOfflineStrip computerName={computer.name} onRetry={app.handleRetry} />}

      {app.installHintVisible && !app.offline && app.activeTab === "sessions" ? (
        <div className="flex shrink-0 items-center justify-between gap-2 border-b border-line-soft bg-sunken px-3.5 py-2 text-eyebrow text-muted-foreground">
          <div className="flex min-w-0 items-center gap-2">
            <Smartphone aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
            <span className="truncate">Add to Home Screen for full-screen access</span>
          </div>
          <Button
            aria-label="Dismiss install hint"
            onClick={() => app.setInstallHintVisible(false)}
            size="icon-xs"
            variant="quiet"
          >
            <X className="size-3" />
          </Button>
        </div>
      ) : null}

      {app.activeTab === "sessions" ? (
        <PhoneSessionList
          filteredSessions={app.filteredSessions}
          offline={app.offline}
          onOpenSession={sessionId => {
            app.setOpenSessionId(sessionId);
            app.setActiveTab("sessions");
          }}
          onSelectProject={app.setProjectFilter}
          projectFilter={app.projectFilter}
          projects={app.projects}
          sessions={sessions}
        />
      ) : null}
      {app.activeTab === "terminal" && app.canOperateTerminal ? (
        <PhoneTerminalPane computer={computer} offline={app.offline} />
      ) : null}
      {app.activeTab === "approvals" ? (
        <PhoneApprovalsPane
          approvals={app.approvals}
          computer={computer}
          onDecide={app.handleDecideApproval}
        />
      ) : null}

      <PhoneDock
        activeTab={app.activeTab}
        canOperateTerminal={app.canOperateTerminal}
        onTabChange={app.setActiveTab}
        pendingApprovalCount={app.pendingApprovalCount}
      />

      <OverlayContainerContext value={overlayHost}>
        <PhoneComputerMenu
          currentComputerName={computer.name}
          onClose={() => app.setComputerMenuOpen(false)}
          onSelectPeer={peer => {
            app.setComputerMenuOpen(false);
            toast.message(`Opening https://${peer.hostname}`);
          }}
          onSignOut={app.handleSignOut}
          open={overlayHost !== null && app.computerMenuOpen}
          peers={peers}
          presentation="sheet"
        />
      </OverlayContainerContext>
    </div>
  );
}

function PhoneDock({
  activeTab,
  canOperateTerminal,
  pendingApprovalCount,
  onTabChange,
}: {
  activeTab: PhoneTabId;
  canOperateTerminal: boolean;
  pendingApprovalCount: number;
  onTabChange: (tab: PhoneTabId) => void;
}) {
  return (
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
        onClick={() => onTabChange("sessions")}
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
          onClick={() => onTabChange("terminal")}
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
        onClick={() => onTabChange("approvals")}
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
  );
}
