import { Ban, RefreshCw, WifiOff } from "@compozy/ui/icons";
import { QueryClientContext } from "@tanstack/react-query";
import { useSelector } from "@xstate/store-react";
import { useContext, useState } from "react";

import { Button, Spinner, cn, toast } from "@compozy/ui";

import { isRemoteComputerId, normalizeComputerId } from "@/lib/computer-scope";
import {
  ComputerPlatformIcon,
  computerRegistryKeys,
  isComputerCounted,
  patchComputer,
} from "@/systems/computers";

import type { ComputerSessionsSource } from "../../hooks/use-computer-sessions-sources";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { SessionFolderDialogState } from "../../hooks/use-session-list-folder-layout";
import { useSessionSidebarState } from "../../hooks/use-session-sidebar-state";
import type { SessionCreateOpenOptions } from "../../lib/session-create-target";
import { SESSION_DROP_CROSS_COMPUTER_REASON } from "../../hooks/use-session-list-dnd";
import type { SessionListGroupBy } from "../../lib/session-list-preferences";
import { sessionDragComputerStore } from "../../lib/session-row-ref";
import {
  computerFoldKey,
  type SessionFoldSet,
  type SessionSubagentFolds,
} from "../../lib/session-sidebar-scope";
import type { SessionThreadBranchChange } from "../../lib/session-thread-branch";
import type { SessionListTarget, WorkspaceFolderPayload } from "../../types";
import { SessionListComputerBody } from "./session-list-computer-body";
import { SessionListComputerNote } from "./session-list-computer-note";
import { SessionListCreateMenu } from "./session-list-create-menu";
import { SessionListGroupToggle } from "./session-list-group-toggle";

const SMALL_ACTION_CLASS = "h-6 shrink-0 px-2 text-badge";

export interface SessionListComputerSectionProps {
  source: ComputerSessionsSource;
  groupBy: SessionListGroupBy;
  search: string;
  collapsedThreads: SessionFoldSet;
  subagentThreads: SessionSubagentFolds;
  currentSessionId?: string;
  onThreadBranchChange: SessionThreadBranchChange;
  onSelectSession: (session: SessionListTarget) => void;
  onNewSession: (options?: SessionCreateOpenOptions) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  onOpenComputerDetails?: (computerId: string) => void;
}

function defaultComputerFolded(source: ComputerSessionsSource): boolean {
  if (source.read === "offline" || source.read === "offline_empty") return true;
  if (source.read === "ok" && source.count === 0) return true;
  return false;
}

function ComputerSectionIdentity({ source }: { source: ComputerSessionsSource }) {
  const quiet =
    source.read === "offline" || source.read === "offline_empty" || source.read === "needs_pro";
  return (
    <span className="flex min-w-0 items-center gap-2">
      <ComputerPlatformIcon
        className="size-3.5 shrink-0"
        iconKind={source.computer.iconKind}
        platform={source.computer.platform}
        size="sm"
        tone="muted"
      />
      <span className="truncate text-small-body font-medium text-fg">{source.name}</span>
      <span
        className={cn(
          "truncate text-eyebrow",
          source.read === "incompatible" ? "text-warning" : quiet ? "text-faint" : "text-subtle"
        )}
      >
        {source.detail}
      </span>
    </span>
  );
}

function ComputerSectionAction({
  source,
  groupBy,
  onNewSession,
  onNewProjectFolder,
  testId,
}: {
  source: ComputerSessionsSource;
  groupBy: SessionListGroupBy;
  onNewSession: () => void;
  onNewProjectFolder?: () => void;
  testId: string;
}) {
  const queryClient = useContext(QueryClientContext);
  const [reconnecting, setReconnecting] = useState(false);

  if (source.read === "connecting") {
    return <Spinner className="mr-1.5 size-3 text-faint" />;
  }
  if (source.read === "offline" || source.read === "offline_empty") {
    return (
      <Button
        className={SMALL_ACTION_CLASS}
        disabled={reconnecting}
        onClick={() => {
          if (!isRemoteComputerId(source.key)) return;
          setReconnecting(true);
          void patchComputer(source.key, { enabled: true })
            .then(() => queryClient?.invalidateQueries({ queryKey: computerRegistryKeys.all }))
            .catch(err => {
              toast.error(err instanceof Error ? err.message : "Failed to reconnect computer.");
            })
            .finally(() => setReconnecting(false));
        }}
        size="xs"
        type="button"
        variant="secondary"
      >
        <RefreshCw aria-hidden="true" className={cn("size-3", reconnecting && "animate-spin")} />
        Reconnect
      </Button>
    );
  }
  if (source.read !== "ok" || !source.canCreate) return null;
  const canCreateProjectFolder = groupBy === "project" && onNewProjectFolder !== undefined;
  return (
    <SessionListCreateMenu
      className="text-subtle"
      folderLabel="New project folder"
      label={{
        session: `New session on ${source.name}`,
        menu: `New session or project folder on ${source.name}`,
      }}
      newFolderUnavailable={
        canCreateProjectFolder ? null : "Group by Project to add folders."
      }
      onNewFolder={canCreateProjectFolder ? onNewProjectFolder : undefined}
      onNewSession={onNewSession}
      size="icon-xs"
      testId={testId}
      variant="quiet"
    />
  );
}

/**
 * One computer section in Separate mode (`F7`, `N10`, `C6`–`C9`, `S1`–`S6`).
 * Folded sections never mount `SessionListComputerBody`, so they cost at most one facets read (`C7`).
 */
export function SessionListComputerSection({
  source,
  groupBy,
  search,
  collapsedThreads,
  subagentThreads,
  currentSessionId,
  onThreadBranchChange,
  onSelectSession,
  onNewSession,
  sessionActions,
  testIdPrefix,
  onOpenComputerDetails,
}: SessionListComputerSectionProps) {
  const sidebar = useSessionSidebarState();
  const dragComputer = useSelector(sessionDragComputerStore, snapshot => snapshot.context.computerId);
  const [hoverRefused, setHoverRefused] = useState(false);
  const [projectFolderDialog, setProjectFolderDialog] =
    useState<SessionFolderDialogState<WorkspaceFolderPayload>>(null);

  const filtering = search.trim() !== "";
  // Search hides sections with 0 matches (`R11`).
  if (filtering && source.read === "ok" && source.count === 0) return null;

  const foldKey = computerFoldKey(source.key);
  const explicit = sidebar.groupFolds.has(foldKey);
  const defaultFolded = defaultComputerFolded(source);
  const collapsed = filtering
    ? false
    : sidebar.foldAll === "collapsed"
      ? !explicit
      : sidebar.foldAll === "groups"
        ? explicit
        : defaultFolded
          ? !explicit
          : explicit;

  const toggle = () => {
    if (!filtering) sidebar.toggleGroup(foldKey);
  };

  const draggingFromOtherComputer =
    dragComputer !== null && normalizeComputerId(dragComputer) !== normalizeComputerId(source.key);
  const refused = hoverRefused && draggingFromOtherComputer;
  const counted = isComputerCounted(source.read);
  const testId = `${testIdPrefix}-computer-${source.key}`;

  return (
    <section
      className={cn(
        "flex flex-col gap-0.5 rounded-md",
        refused && "cursor-not-allowed bg-surface-2/60"
      )}
      data-collapsed={collapsed ? "true" : undefined}
      data-computer-section={source.key}
      data-testid={testId}
      onDragLeave={event => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
          setHoverRefused(false);
        }
      }}
      onDragOver={event => {
        if (!draggingFromOtherComputer) return;
        event.preventDefault();
        event.dataTransfer.dropEffect = "none";
        if (!hoverRefused) setHoverRefused(true);
      }}
      onDrop={() => setHoverRefused(false)}
    >
      <div className="sticky top-0 z-10 flex min-h-8 min-w-0 items-center gap-1 rounded-sm bg-sunken">
        <SessionListGroupToggle
          collapsed={collapsed}
          count={counted ? source.count : null}
          identity={<ComputerSectionIdentity source={source} />}
          leading={{ needsYou: source.needsYou, working: source.working, slot: "session-group" }}
          onToggle={toggle}
          testId={`${testId}-toggle`}
        />
        <ComputerSectionAction
          groupBy={groupBy}
          onNewProjectFolder={
            source.canOperate && groupBy === "project"
              ? () => {
                  if (collapsed) sidebar.toggleGroup(foldKey);
                  setProjectFolderDialog({ kind: "create" });
                }
              : undefined
          }
          onNewSession={() => onNewSession({ computerId: source.key, fixedComputer: true })}
          source={source}
          testId={testId}
        />
      </div>
      {refused ? (
        <SessionListComputerNote icon={Ban} testId={`${testId}-drop-refused`}>
          {SESSION_DROP_CROSS_COMPUTER_REASON}
        </SessionListComputerNote>
      ) : null}
      {filtering && source.read === "offline" ? (
        <SessionListComputerNote icon={WifiOff} testId={`${testId}-offline-search-note`}>
          Matches from the rows seen before it went offline.
        </SessionListComputerNote>
      ) : null}
      {collapsed ? null : (
        <div className="pl-2.5">
          <SessionListComputerBody
            collapsedThreads={collapsedThreads}
            currentSessionId={currentSessionId}
            onNewSession={onNewSession}
            onOpenComputerDetails={onOpenComputerDetails}
            onProjectFolderDialogChange={setProjectFolderDialog}
            onSelectSession={onSelectSession}
            onThreadBranchChange={onThreadBranchChange}
            projectFolderDialog={projectFolderDialog}
            scoped={false}
            search={search}
            sessionActions={sessionActions}
            source={source}
            subagentThreads={subagentThreads}
            testIdPrefix={testIdPrefix}
          />
        </div>
      )}
    </section>
  );
}
