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

import {
  Button,
  cn,
  CodeBlock,
  Empty,
  Logo,
  Pill,
  PillCount,
  StateGlyph,
  StatusDot,
  Surface,
  toast,
} from "@compozy/ui";

import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import { KeyFingerprint } from "@/systems/computers/components/key-fingerprint";
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
  PHONE_TERMINAL_OUTPUT,
} from "@/systems/computers/mocks/remote-computers-fixtures-phone";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

import { useIsWideBrowser } from "./phone-frame";
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

function sessionStateLabel(state: UnifiedSessionFixture["state"]): string {
  switch (state) {
    case "attention":
      return "Needs you";
    case "running":
      return "Working";
    case "done":
      return "Done";
    case "idle":
      return "Idle";
  }
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
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex shrink-0 items-center gap-1.5 overflow-x-auto border-b border-line-soft px-3.5 py-2">
        <Pill
          active={projectFilter === "all"}
          onClick={() => setProjectFilter("all")}
          render={<button type="button" />}
          size="sm"
          tone="neutral"
        >
          All · {sessions.length}
        </Pill>
        {projects.map(project => (
          <Pill
            active={projectFilter === project}
            key={project}
            onClick={() => setProjectFilter(project)}
            render={<button type="button" />}
            size="sm"
            tone="neutral"
          >
            {project}
          </Pill>
        ))}
      </div>

      <div
        className={cn(
          "flex min-h-0 flex-1 flex-col divide-y divide-line-soft overflow-y-auto",
          offline && "opacity-70"
        )}
      >
        {filteredSessions.map(item => {
          const needsYou = item.state === "attention";
          const isSelected = isWide && activeTab === "sessions" && openSessionId === item.id;
          return (
            <button
              aria-current={isSelected ? "true" : undefined}
              className={cn(
                "flex min-h-15 w-full items-start gap-3 px-4 py-3 text-left transition-colors hover:bg-surface-2 focus-visible:shadow-focus-inset focus-visible:outline-none",
                isSelected && "bg-selected hover:bg-selected"
              )}
              key={item.id}
              onClick={() => {
                setOpenSessionId(item.id);
                setActiveTab("sessions");
              }}
              type="button"
            >
              <span className="mt-1 shrink-0">
                <StateGlyph size="md" state={offline ? "idle" : item.state} still={offline} />
              </span>
              <div className="flex min-w-0 flex-1 flex-col gap-1">
                <div className="flex items-baseline justify-between gap-2">
                  <span
                    className={cn(
                      "min-w-0 truncate text-body text-fg",
                      (needsYou || isSelected) && "font-medium text-fg-strong"
                    )}
                  >
                    {item.title}
                  </span>
                  <span className="shrink-0 font-mono text-micro text-subtle tabular-nums">
                    {item.updatedLabel}
                  </span>
                </div>
                <div className="flex items-center gap-1.5 text-eyebrow text-subtle">
                  <WorkspaceBadge
                    identity={{ id: item.project, kind: "workspace", name: item.project }}
                  />
                  <span aria-hidden="true">·</span>
                  <span className={cn(needsYou && !offline && "font-medium text-attn")}>
                    {offline ? "Offline" : sessionStateLabel(item.state)}
                  </span>
                  <span aria-hidden="true">·</span>
                  <span className="truncate">{item.model}</span>
                </div>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );

  const renderTerminalPane = () => (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div
        className={cn(
          "mx-auto flex w-full max-w-3xl flex-col gap-3",
          isWide ? "px-6 py-6" : "px-3.5 py-3.5"
        )}
      >
        <div className="flex items-center justify-between gap-2">
          <div className="flex flex-col">
            <span className="text-body font-medium text-fg-strong">zsh on {computer.name}</span>
            <span className="font-mono text-micro text-subtle">/Users/pedro/code/api</span>
          </div>
          <Pill form="plain" tone={offline ? "warning" : "success"}>
            <Pill.Dot />
            {offline ? "Disconnected" : "Live"}
          </Pill>
        </div>
        <CodeBlock
          code={PHONE_TERMINAL_OUTPUT.join("\n")}
          copyable
          density="compact"
          language="bash"
          wrapLines
        />
        <p className="text-eyebrow text-subtle">
          Commands run directly on {computer.name} under your user account. Output stays buffered if
          your connection drops briefly.
        </p>
      </div>
    </div>
  );

  const renderApprovalsPane = () => (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div
        className={cn(
          "mx-auto flex w-full max-w-xl flex-col gap-3",
          isWide ? "px-6 py-6" : "px-3.5 py-3.5"
        )}
      >
        {approvals.length === 0 ? (
          <Empty
            description={`New device requests for ${computer.name} appear here.`}
            icon={Bell}
            size="compact"
            title="No pending approvals"
          />
        ) : (
          approvals.map(item => (
            <Surface className="flex flex-col gap-3 p-4" key={item.id} variant="card">
              <div className="flex items-start justify-between gap-2">
                <div className="flex min-w-0 flex-col gap-0.5">
                  <span className="truncate text-body font-medium text-fg-strong">
                    {item.deviceName}
                  </span>
                  <span className="text-eyebrow text-subtle">
                    Wants to access {item.computerName} · {item.createdLabel}
                  </span>
                </div>
                <Pill
                  form="tint"
                  tone={
                    item.state === "approved"
                      ? "success"
                      : item.state === "denied"
                        ? "destructive"
                        : "warning"
                  }
                >
                  {item.state === "approved"
                    ? "Approved"
                    : item.state === "denied"
                      ? "Denied"
                      : "Waiting"}
                </Pill>
              </div>

              <KeyFingerprint
                columns={3}
                label="Device fingerprint"
                layout="grid"
                value={item.keyFingerprint}
              />

              {item.canDecide ? (
                <div className="grid grid-cols-2 gap-2">
                  <Button
                    disabled={item.state !== "pending"}
                    onClick={() => {
                      setApprovals(current =>
                        current.map(entry =>
                          entry.id === item.id ? { ...entry, state: "denied" } : entry
                        )
                      );
                      toast(`Denied ${item.deviceName}`);
                    }}
                    size="lg"
                    variant="outline"
                  >
                    Deny
                  </Button>
                  <Button
                    disabled={item.state !== "pending"}
                    onClick={() => {
                      setApprovals(current =>
                        current.map(entry =>
                          entry.id === item.id ? { ...entry, state: "approved" } : entry
                        )
                      );
                      toast.success(`Approved ${item.deviceName}`);
                    }}
                    size="lg"
                  >
                    Approve
                  </Button>
                </div>
              ) : (
                <p className="text-eyebrow text-subtle">
                  Approve on {item.computerName}, or from a device it already trusts.
                </p>
              )}
            </Surface>
          ))
        )}
      </div>
    </div>
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
            renderTerminalPane()
          ) : activeTab === "approvals" ? (
            renderApprovalsPane()
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
      {activeTab === "terminal" && canOperateTerminal ? renderTerminalPane() : null}
      {activeTab === "approvals" ? renderApprovalsPane() : null}

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
        open={computerMenuOpen}
        peers={peers}
        presentation="sheet"
      />
    </div>
  );
}
