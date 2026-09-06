import { Eraser, Pencil, Square, Trash2 } from "lucide-react";
import { useRef, useState } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Spinner,
  TopbarOverflowIcon,
  useTopbarSlot,
} from "@compozy/ui";

import { SessionPrimaryAction } from "../components/session-primary-action";
import { SessionPanelToggle } from "../components/session-panel-toggle";

import type { WorktreePayload } from "@/systems/workspace";

import { getSessionDisplayTitle } from "../lib/session-display-title";
import { isSessionRunning, isUserControllableSession } from "../lib/session-running";
import type { SessionPayload } from "../types";
import { SessionStatusLine } from "../components/session-status-line";
import { SessionWorktreeBindingChip } from "../components/session-worktree-binding-chip";
import { SessionTransportContext } from "../lib/session-transcript-thread-context-value";
import { useSessionTransportState } from "./use-session-transcript-thread-messages";

interface SessionTopbarWorktreeBinding {
  worktreeId: string;
  worktree: WorktreePayload | undefined;
  onOpenContext?: () => void;
  onResolve?: () => void;
}

interface UseSessionTopbarSlotInput {
  /** Absent for an unbound session — absence is the signal, not a placeholder. */
  worktreeBinding?: SessionTopbarWorktreeBinding;
  session: SessionPayload;
  isDeleting: boolean;
  isRenaming: boolean;
  isStopping: boolean;
  isResuming: boolean;
  isUnarchiving: boolean;
  isClearing: boolean;
  canClear: boolean;
  inspectorOpen: boolean;
  sidebarOpen: boolean;
  /** The one state-gated goal action (Pause/Resume/Approve/Clear) riding the head. */
  goalAction?: React.ReactNode;
  /** The connection chip (S4): mounts in the status slot's reserved trailing position, before the actions. */
  transportChip?: React.ReactNode;
  onInspectorToggle: () => void;
  onSidebarToggle: () => void;
  onDelete: () => void;
  onRename: () => void;
  onStop: () => void;
  onResume: () => void;
  onUnarchive: () => void;
  onClear: () => void;
}

function sessionTopbarActions({
  session,
  isStopping,
  isResuming,
  isUnarchiving,
  isDeleting,
  isRenaming,
}: UseSessionTopbarSlotInput) {
  const isActive = session.state === "active" || session.state === "starting";
  const isArchived = session.archived_at !== null;
  const lifecycleControllable = isUserControllableSession(session);
  const canResume =
    session.archived_at === null &&
    session.attachable === true &&
    lifecycleControllable &&
    !isSessionRunning(session);
  const showUnarchiveAction = lifecycleControllable && isArchived;
  const showStopAction = lifecycleControllable && isActive && !canResume;
  const controlsBusy = isStopping || isResuming || isUnarchiving || isDeleting || isRenaming;

  return {
    isActive,
    lifecycleControllable,
    canResume,
    showUnarchiveAction,
    showStopAction,
    controlsBusy,
  };
}

type SessionTopbarActions = ReturnType<typeof sessionTopbarActions>;

function useSessionTopbarOverflow(input: UseSessionTopbarSlotInput, actions: SessionTopbarActions) {
  const {
    onRename,
    isRenaming,
    isStopping,
    onStop,
    canClear,
    isClearing,
    onClear,
    isDeleting,
    onDelete,
  } = input;
  const { lifecycleControllable, controlsBusy, isActive, canResume } = actions;
  const [overflowOpen, setOverflowOpen] = useState(false);
  const renameRequested = useRef(false);
  return lifecycleControllable ? (
    <DropdownMenu
      open={overflowOpen}
      onOpenChange={setOverflowOpen}
      onOpenChangeComplete={open => {
        if (open || !renameRequested.current) return;
        renameRequested.current = false;
        onRename();
      }}
    >
      <DropdownMenuTrigger
        aria-label="More actions"
        data-testid="session-topbar-overflow"
        render={
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            className="size-11 focus-visible:shadow-focus-inset"
          />
        }
      >
        <TopbarOverflowIcon aria-hidden="true" className="size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" data-testid="session-topbar-overflow-menu">
        <DropdownMenuItem
          data-testid="rename-button"
          disabled={controlsBusy}
          onClick={() => {
            renameRequested.current = true;
            setOverflowOpen(false);
          }}
        >
          {isRenaming ? <Spinner className="size-3" /> : <Pencil className="size-3" />}
          Rename session
        </DropdownMenuItem>
        {isActive && canResume ? (
          <DropdownMenuItem
            data-testid="stop-menu-item"
            disabled={controlsBusy && !isStopping}
            onClick={onStop}
          >
            {isStopping ? <Spinner className="size-3" /> : <Square className="size-3" />}
            Stop session
          </DropdownMenuItem>
        ) : null}
        <DropdownMenuItem
          data-testid="composer-clear-button"
          disabled={!canClear || isClearing}
          onClick={onClear}
        >
          {isClearing ? <Spinner className="size-3" /> : <Eraser className="size-3" />}
          Clear conversation
        </DropdownMenuItem>
        <DropdownMenuItem
          data-testid="delete-button"
          disabled={controlsBusy}
          onClick={onDelete}
          variant="destructive"
        >
          {isDeleting ? <Spinner className="size-3" /> : <Trash2 className="size-3" />}
          Delete session
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  ) : null;
}

/** Publishes lifecycle actions into the owning session window's topbar. */
export function useSessionTopbarSlot(input: UseSessionTopbarSlotInput): void {
  const { session, worktreeBinding, transportChip } = input;
  const actions = sessionTopbarActions(input);
  const { isActive } = actions;
  const overflow = useSessionTopbarOverflow(input, actions);
  // The slot consumer (the OS head) renders outside this window's session
  // runtime provider, so a node that reads the transport — the chip — would
  // see the default live snapshot there. The publisher runs inside the
  // provider: it carries the window's own snapshot with the node, per window,
  // no second stream (task_06 VC-01..05).
  const transport = useSessionTransportState();

  useTopbarSlot({
    glyph: (
      <span
        className={
          isActive
            ? "size-[7px] rounded-full bg-accent motion-safe:animate-pulse"
            : "size-[7px] rounded-full bg-faint"
        }
      />
    ),
    glyphPresentation: "state",
    crumb: getSessionDisplayTitle(session),
    status: (
      <span className="flex min-w-0 items-center gap-2">
        <SessionStatusLine session={session} showState={false} />
        {/* OQ7: the binding chip mounts here; the exit control never does. */}
        {worktreeBinding ? (
          <SessionWorktreeBindingChip
            onOpenContext={worktreeBinding.onOpenContext}
            onResolve={worktreeBinding.onResolve}
            worktree={worktreeBinding.worktree}
            worktreeId={worktreeBinding.worktreeId}
          />
        ) : null}
        {transportChip ? (
          <SessionTransportContext.Provider value={transport}>
            {transportChip}
          </SessionTransportContext.Provider>
        ) : null}
      </span>
    ),
    actions: (
      <>
        <SessionPanelToggle
          panel="sidebar"
          open={input.sidebarOpen}
          onToggle={input.onSidebarToggle}
        />
        {input.goalAction}
        <SessionPrimaryAction {...input} {...actions} />
        <SessionPanelToggle
          panel="inspector"
          open={input.inspectorOpen}
          onToggle={input.onInspectorToggle}
        />
      </>
    ),
    overflow,
  });
}
