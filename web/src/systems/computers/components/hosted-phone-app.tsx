import {
  ChevronDown,
  MessageSquare,
  RefreshCw,
  ShieldCheck,
  Smartphone,
  TerminalSquare,
  WifiOff,
  X,
} from "@compozy/ui/icons";

import { Button, cn, Logo, PillCount, Spinner, StatusDot } from "@compozy/ui";

import { useHostedPhoneApp, type HostedPhoneTab } from "../hooks/use-hosted-phone-app";
import { ComputerPlatformIcon } from "./computer-platform-icon";
import { HostedPhoneApprovalsPane, HostedPhoneSessionList } from "./hosted-phone-panes";
import { HostedPhoneSessionView } from "./hosted-phone-session-view";
import { HostedPhoneTerminalPane } from "./hosted-phone-terminal-pane";
import { PhoneComputerMenu } from "./phone-computer-menu";

function PhoneTabButton({
  active,
  badgeCount,
  icon,
  label,
  onClick,
  testId,
}: {
  active: boolean;
  badgeCount?: number;
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
  testId: string;
}) {
  return (
    <button
      aria-current={active ? "page" : undefined}
      className={cn(
        "relative flex flex-1 flex-col items-center justify-center gap-0.5 py-2 text-eyebrow transition-colors focus-visible:shadow-focus-ring focus-visible:outline-none",
        active ? "font-medium text-fg-strong" : "text-muted-foreground hover:text-fg"
      )}
      data-testid={testId}
      onClick={onClick}
      type="button"
    >
      <span className="relative inline-flex items-center justify-center">
        {icon}
        {badgeCount && badgeCount > 0 ? (
          <PillCount className="absolute -top-1.5 -right-2.5" count={badgeCount} />
        ) : null}
      </span>
      <span>{label}</span>
    </button>
  );
}

function HostedPhoneMainPane({
  computerName,
  model,
}: {
  computerName: string;
  model: ReturnType<typeof useHostedPhoneApp>;
}) {
  if (model.selectedSession) {
    return (
      <HostedPhoneSessionView
        canOperateTerminal={model.canOperateTerminal}
        onBack={model.closeSession}
        onOpenTerminal={model.canOperateTerminal ? model.openTerminalFromSession : undefined}
        projectName={model.selectedSessionProjectName}
        session={model.selectedSession}
      />
    );
  }

  if (model.activeTab === "approvals") {
    return (
      <HostedPhoneApprovalsPane
        approvals={model.approvals}
        onApprove={model.approveItem}
        onDeny={model.denyItem}
        onOpenSession={model.openSessionById}
        pendingIds={model.pendingDecisionIds}
      />
    );
  }

  if (model.activeTab === "terminal" && model.canOperateTerminal) {
    return (
      <HostedPhoneTerminalPane
        projectLabel={model.terminalProjectLabel}
        workspaceId={model.terminalWorkspaceId}
      />
    );
  }

  return (
    <HostedPhoneSessionList
      computerName={computerName}
      error={model.sessionsError}
      loading={model.sessionsLoading}
      onOpenSession={model.openSession}
      onRetry={model.retrySessions}
      sessions={model.sessions}
      workspaceNameById={model.workspaceNameById}
    />
  );
}

/**
 * Compact phone layout (`< 1024px`) for the hosted browser SPA (`https://<slug>.compozy.run`, U8).
 *
 * Exposes Sessions, Approvals, and (when granted `terminal:operate`) Terminal tabs,
 * plus the computer switcher sheet (`PhoneComputerMenu`), offline/reconnecting strip,
 * and dismissible install-as-app hint.
 */
export function HostedPhoneApp() {
  const model = useHostedPhoneApp();
  const hosted = model.hosted;
  if (!hosted) return null;

  const { computer, peers, reconnecting, selectPeer, signOut, retryConnection } = hosted;

  const handleSelectTab = (tab: HostedPhoneTab) => {
    model.closeSession();
    model.setActiveTab(tab);
  };

  return (
    <div
      className="relative flex h-dvh flex-col overflow-hidden bg-background text-foreground"
      data-testid="hosted-phone-app"
    >
      <header className="flex h-12 shrink-0 items-center justify-between gap-2 border-b border-line-soft bg-rail px-3">
        <div className="flex min-w-0 items-center gap-2">
          <Logo className="size-5 shrink-0" decorative variant="symbol" />
          <button
            aria-label={`Connected to ${computer.name}. Switch computer`}
            className="inline-flex h-8 max-w-52 items-center gap-1.5 rounded-pill border border-line bg-surface px-2.5 text-small-body font-medium text-fg transition-colors hover:bg-surface-2 focus-visible:shadow-focus-ring focus-visible:outline-none"
            data-testid="phone-computer-pill"
            onClick={() => model.setComputerMenuOpen(true)}
            type="button"
          >
            <ComputerPlatformIcon
              className="size-3.5 shrink-0"
              iconKind={computer.iconKind}
              platform={computer.platform}
              size="sm"
              tone="muted"
            />
            <span className="truncate">{computer.name}</span>
            {reconnecting ? (
              <Spinner aria-hidden="true" className="size-2.5 text-warning" />
            ) : (
              <StatusDot size="sm" tone="success" />
            )}
            <ChevronDown aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
          </button>
        </div>
      </header>

      {reconnecting ? (
        <div
          className="flex shrink-0 items-center justify-between gap-2 border-b border-line bg-warning-tint px-3 py-2 text-eyebrow text-warning"
          data-testid="phone-offline-strip"
          role="status"
        >
          <span className="inline-flex min-w-0 items-center gap-1.5">
            <WifiOff aria-hidden="true" className="size-3.5 shrink-0" />
            <span className="truncate">Reconnecting to {computer.name}…</span>
          </span>
          <Button onClick={retryConnection} size="xs" type="button" variant="ghost">
            <RefreshCw aria-hidden="true" className="size-3" />
            Retry
          </Button>
        </div>
      ) : null}

      {model.installHintVisible && !model.selectedSession ? (
        <div
          className="flex shrink-0 items-center justify-between gap-2 border-b border-line-soft bg-sunken px-3 py-2 text-eyebrow text-muted-foreground"
          data-testid="phone-install-hint"
        >
          <span className="inline-flex min-w-0 items-center gap-1.5">
            <Smartphone aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
            <span className="truncate">Add to Home Screen for quick access to {computer.name}</span>
          </span>
          <Button
            aria-label="Dismiss install hint"
            onClick={model.dismissInstallHint}
            size="icon-xs"
            type="button"
            variant="ghost"
          >
            <X className="size-3.5" />
          </Button>
        </div>
      ) : null}

      <main className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <HostedPhoneMainPane computerName={computer.name} model={model} />
      </main>

      <nav
        aria-label="Phone navigation"
        className="flex shrink-0 items-stretch border-t border-line-soft bg-rail pb-[env(safe-area-inset-bottom,0px)]"
        data-testid="phone-bottom-dock"
      >
        <PhoneTabButton
          active={model.activeTab === "sessions"}
          icon={<MessageSquare aria-hidden="true" className="size-4" />}
          label="Sessions"
          onClick={() => handleSelectTab("sessions")}
          testId="phone-tab-sessions"
        />
        <PhoneTabButton
          active={model.activeTab === "approvals"}
          badgeCount={model.approvals.length}
          icon={<ShieldCheck aria-hidden="true" className="size-4" />}
          label="Approvals"
          onClick={() => handleSelectTab("approvals")}
          testId="phone-tab-approvals"
        />
        {model.canOperateTerminal ? (
          <PhoneTabButton
            active={model.activeTab === "terminal"}
            icon={<TerminalSquare aria-hidden="true" className="size-4" />}
            label="Terminal"
            onClick={() => handleSelectTab("terminal")}
            testId="phone-tab-terminal"
          />
        ) : null}
      </nav>

      <PhoneComputerMenu
        currentComputerName={computer.name}
        onClose={() => model.setComputerMenuOpen(false)}
        onSelectPeer={peer => {
          model.setComputerMenuOpen(false);
          selectPeer(peer);
        }}
        onSignOut={() => {
          model.setComputerMenuOpen(false);
          signOut();
        }}
        open={model.computerMenuOpen}
        peers={peers}
        presentation="sheet"
      />
    </div>
  );
}
