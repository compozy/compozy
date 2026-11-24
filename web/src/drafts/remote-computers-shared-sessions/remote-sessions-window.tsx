import {
  FileDiff,
  FolderOpen,
  MessageSquare,
  RefreshCw,
  Sparkles,
  SquareTerminal,
  TriangleAlert,
  WifiOff,
} from "@compozy/ui/icons";
import { useState, type ReactNode } from "react";

import {
  Button,
  Spinner,
  StateGlyph,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  type TopbarSlotValue,
} from "@compozy/ui";

import { ComputerChip } from "@/systems/computers/components/computer-chip";
import type { ComputerFixture } from "@/systems/computers/mocks/remote-computers-fixtures";
import type {
  RemoteSessionRowEntry,
  TranscriptTurnFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures-sessions";
import { SessionViewToggle } from "@/systems/session/components/session-board/session-view-toggle";
import { SessionPanelToggle } from "@/systems/session/components/session-panel-toggle";
import { SessionPrimaryAction } from "@/systems/session/components/session-primary-action";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

import { DraftShell } from "../always-on-agents-shared/draft-shell";
import {
  AgentTurn,
  DraftComposer,
  TranscriptColumn,
  UserTurn,
  WorkingLine,
} from "../always-on-agents-shared/transcript";
import {
  RemoteSessionsSidebar,
  type PerComputerErrorBlock,
  type PerComputerLoadingBlock,
  type PerComputerPaginationBlock,
} from "./remote-sessions-sidebar";

export interface RemoteSessionsWindowProps {
  sessions: readonly RemoteSessionRowEntry[];
  activeSession: RemoteSessionRowEntry;
  /** Computer object for the active session when remote; `null` when local. */
  activeComputer: ComputerFixture | null;
  transcript: readonly TranscriptTurnFixture[];
  /** Shows the working streaming indicator under the transcript. */
  isWorking?: boolean;
  /** Opens the header ComputerChip popover initially. */
  computerChipOpen?: boolean;
  /** Shows the `remote_entitlement_unavailable` Pro-ended mid-session pause state. */
  entitlementPaused?: boolean;
  initialGroupBy?: "none" | "project";
  loadingComputers?: readonly PerComputerLoadingBlock[];
  failedComputers?: readonly PerComputerErrorBlock[];
  perComputerMore?: readonly PerComputerPaginationBlock[];
  /** Dialogs or search overlays rendered over the full window. */
  overlay?: ReactNode;
  onViewChange?: (view: "list" | "kanban") => void;
  onNewSession?: () => void;
  /**
   * Hosted-route web app (U8): the computer that serves this browser tab. It is "this computer"
   * to the app, so its sessions stay local and copy names it instead of "This computer".
   */
  hostName?: string;
  /** Hosted-route web app: the tab lost its connection to `hostName` (read-only, reconnecting). */
  hostConnectionLost?: boolean;
  /** Hosted-route web app: the browser device's scopes allow the terminal. */
  canOperateTerminal?: boolean;
  /** Replaces the session `ComputerChip` in the window topbar (the hosted computer picker). */
  topbarContext?: ReactNode;
  /** Project chip in the system bar. */
  scope?: string;
  /** Root sizing override, e.g. `h-full` inside a browser frame. */
  className?: string;
}

function RemoteSessionToolButton({
  label,
  disabled,
  children,
}: {
  label: string;
  disabled: boolean;
  children: ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            aria-label={label}
            disabled={disabled}
            size="icon-sm"
            type="button"
            variant="quiet"
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

function RemoteSessionActions({
  remoteComputerName,
  hasTerminalScope,
  hasFilesScope,
  disabled,
  isRunning,
  railOpen,
  onToggleRail,
}: {
  remoteComputerName: string | undefined;
  hasTerminalScope: boolean;
  hasFilesScope: boolean;
  disabled: boolean;
  isRunning: boolean;
  railOpen: boolean;
  onToggleRail: () => void;
}) {
  const suffix = remoteComputerName ? ` on ${remoteComputerName}` : "";
  return (
    <>
      {hasTerminalScope && (
        <RemoteSessionToolButton disabled={disabled} label={`Terminal${suffix}`}>
          <SquareTerminal aria-hidden="true" className="size-4" />
        </RemoteSessionToolButton>
      )}
      {hasFilesScope && (
        <>
          <RemoteSessionToolButton disabled={disabled} label={`Files${suffix}`}>
            <FolderOpen aria-hidden="true" className="size-4" />
          </RemoteSessionToolButton>
          <RemoteSessionToolButton disabled={disabled} label={`Review${suffix}`}>
            <FileDiff aria-hidden="true" className="size-4" />
          </RemoteSessionToolButton>
        </>
      )}
      {!disabled && isRunning && (
        <SessionPrimaryAction
          controlsBusy={false}
          isStopping={false}
          isUnarchiving={false}
          onStop={() => undefined}
          onUnarchive={() => undefined}
          showStopAction
          showUnarchiveAction={false}
        />
      )}
      <SessionPanelToggle onToggle={onToggleRail} open={railOpen} panel="sidebar" />
      <SessionPanelToggle onToggle={() => undefined} open={false} panel="inspector" />
    </>
  );
}

function RemoteSessionComposerBanner({
  computerName,
  isOffline,
  entitlementPaused,
  reconnecting,
  onReconnect,
}: {
  computerName: string;
  isOffline: boolean;
  entitlementPaused: boolean;
  reconnecting: boolean;
  onReconnect: () => void;
}) {
  if (isOffline) {
    return (
      <div
        className="flex items-center justify-between gap-3 rounded-md border border-warning/25 bg-warning-tint px-3 py-1.5 text-small-body"
        data-testid="remote-offline-composer-banner"
        role="status"
      >
        <span className="flex min-w-0 items-center gap-2 text-fg-2">
          <WifiOff aria-hidden="true" className="size-3.5 shrink-0 text-warning" />
          <span className="truncate">
            <span className="font-medium text-fg">{computerName} is offline.</span> Showing the last
            messages loaded in this session.
          </span>
        </span>
        <Button
          className="shrink-0"
          disabled={reconnecting}
          onClick={onReconnect}
          size="xs"
          type="button"
          variant="secondary"
        >
          {reconnecting ? (
            <Spinner aria-hidden="true" data-icon="inline-start" />
          ) : (
            <RefreshCw aria-hidden="true" data-icon="inline-start" />
          )}
          {reconnecting ? "Reconnecting…" : "Reconnect"}
        </Button>
      </div>
    );
  }
  if (!entitlementPaused) return null;
  return (
    <div
      className="flex items-center justify-between gap-3 rounded-md border border-line bg-surface-2 px-3 py-1.5 text-small-body"
      data-testid="remote-entitlement-paused-banner"
      role="status"
    >
      <span className="flex min-w-0 items-center gap-2 text-fg-2">
        <TriangleAlert aria-hidden="true" className="size-3.5 shrink-0 text-warning" />
        <span className="truncate">
          <span className="font-medium text-fg">Remote access paused:</span> your Pro plan ended.
          Running agents on {computerName} keep running.
        </span>
      </span>
      <Button className="shrink-0" size="xs" type="button" variant="default">
        <Sparkles aria-hidden="true" data-icon="inline-start" />
        Upgrade
      </Button>
    </div>
  );
}

function RemoteSessionTranscript({
  transcript,
  workingLabel,
}: {
  transcript: readonly TranscriptTurnFixture[];
  workingLabel: string | null;
}) {
  return (
    <TranscriptColumn>
      {transcript.map(turn =>
        turn.role === "user" ? (
          <UserTurn key={turn.id}>{turn.text}</UserTurn>
        ) : (
          <div className="flex flex-col gap-2.5" key={turn.id}>
            {turn.toolLine && (
              <div className="inline-flex w-fit items-center gap-2 rounded-md border border-line-soft bg-sunken px-3 py-1.5 font-mono text-eyebrow text-muted-foreground">
                <StateGlyph size="sm" state="done" />
                <span>{turn.toolLine}</span>
              </div>
            )}
            <AgentTurn>{turn.text}</AgentTurn>
          </div>
        )
      )}
      {workingLabel && <WorkingLine who={workingLabel} />}
    </TranscriptColumn>
  );
}

/**
 * Full CompozyOS Sessions window for remote-computer review (Screens 05 & 06):
 * - Composes `DraftShell`, `RemoteSessionsSidebar`, real window `Topbar` with `ComputerChip`
 * - Truthfully omits client-OS actions (`Open in editor`, `Reveal in Finder`) on remote sessions
 * - Truthfully omits `Terminal` when device lacks `terminal:operate` scope (US-011 EC-1)
 * - Renders read-only offline banner + disabled composer when the remote computer is offline
 */
export function RemoteSessionsWindow({
  sessions,
  activeSession,
  activeComputer,
  transcript,
  isWorking = false,
  computerChipOpen = false,
  entitlementPaused = false,
  initialGroupBy = "none",
  loadingComputers,
  failedComputers,
  perComputerMore,
  overlay,
  onViewChange,
  onNewSession,
  hostName,
  hostConnectionLost = false,
  canOperateTerminal,
  topbarContext,
  scope = "All projects",
  className,
}: RemoteSessionsWindowProps) {
  const [railOpen, setRailOpen] = useState(true);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [reconnecting, setReconnecting] = useState(false);

  const isRemote = activeComputer !== null;
  const isOffline = activeComputer?.status === "offline" || hostConnectionLost;
  const hasTerminalScope =
    canOperateTerminal ?? (!isRemote || activeComputer.scopes.includes("terminal:operate"));
  const hasFilesScope = !isRemote || activeComputer.scopes.includes("files:read");
  const composerDisabled = isOffline || entitlementPaused;

  const computerName = activeComputer?.name ?? hostName ?? "This computer";
  const reconnect = () => setReconnecting(true);

  const topbar: TopbarSlotValue = {
    glyph: (
      <StateGlyph
        size="sm"
        state={
          isOffline
            ? "idle"
            : activeSession.state === "running"
              ? "running"
              : activeSession.state === "attention"
                ? "attention"
                : "done"
        }
      />
    ),
    glyphPresentation: "state",
    crumb: activeSession.title,
    context:
      topbarContext ??
      (activeComputer ? (
        <ComputerChip
          computer={
            entitlementPaused
              ? { ...activeComputer, status: "needs_pro" }
              : reconnecting
                ? { ...activeComputer, status: "connecting" }
                : activeComputer
          }
          defaultOpen={computerChipOpen}
          onOpenDetails={() => undefined}
          onReconnect={isOffline ? reconnect : undefined}
          onUpgrade={entitlementPaused ? () => undefined : undefined}
        />
      ) : undefined),
    actions: (
      <RemoteSessionActions
        disabled={composerDisabled}
        hasFilesScope={hasFilesScope}
        hasTerminalScope={hasTerminalScope}
        isRunning={activeSession.state === "running"}
        onToggleRail={() => setRailOpen(open => !open)}
        railOpen={railOpen}
        remoteComputerName={activeComputer?.name}
      />
    ),
    overflow: <SessionViewToggle onValueChange={next => onViewChange?.(next)} value="list" />,
  };

  return (
    <DraftShell
      app="sessions"
      glyph={<MessageSquare />}
      leading={
        <WorkspaceBadge
          identity={{ kind: "project", name: activeSession.project }}
          variant="chrome"
        />
      }
      overlay={overlay}
      className={className}
      scope={scope}
      sessionBadge={2}
      title={activeSession.title}
      topbar={topbar}
    >
      <div className="flex min-h-0 min-w-0 flex-1">
        {railOpen ? (
          <RemoteSessionsSidebar
            activeSessionId={selectedId ?? activeSession.id}
            failedComputers={failedComputers}
            initialGroupBy={initialGroupBy}
            loadingComputers={loadingComputers}
            onNewSession={onNewSession}
            onSelectSession={item => setSelectedId(item.id)}
            perComputerMore={perComputerMore}
            sessions={sessions}
          />
        ) : null}

        <div className="flex min-w-0 flex-1 flex-col border-l border-line bg-canvas">
          <RemoteSessionTranscript
            transcript={transcript}
            workingLabel={isWorking && !composerDisabled ? `Claude on ${computerName}` : null}
          />

          <DraftComposer
            above={
              composerDisabled ? (
                <RemoteSessionComposerBanner
                  computerName={computerName}
                  entitlementPaused={entitlementPaused}
                  isOffline={isOffline}
                  onReconnect={reconnect}
                  reconnecting={reconnecting}
                />
              ) : undefined
            }
            disabled={composerDisabled}
            placeholder={
              isOffline
                ? `${computerName} is offline`
                : entitlementPaused
                  ? `${computerName} needs Pro to send messages`
                  : `Message Claude on ${computerName}`
            }
            runtime={{
              provider: activeSession.provider,
              model: activeSession.provider === "codex" ? "gpt-6-astra" : "claude-opus-5-5",
            }}
          />
        </div>
      </div>
    </DraftShell>
  );
}
