import { Eraser, PanelRight, Play, Square, Trash2 } from "lucide-react";

import {
  Button,
  cn,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Spinner,
  TopbarOverflowIcon,
  useTopbarSlot,
} from "@agh/ui";

import { getSessionDisplayTitle } from "../lib/session-display-title";
import { isSessionRunning, isUserControllableSession } from "../lib/session-running";
import type { SessionPayload } from "../types";
import { SessionStatusLine } from "../components/session-status-line";

interface UseSessionTopbarSlotInput {
  session: SessionPayload;
  isDeleting: boolean;
  isStopping: boolean;
  isResuming: boolean;
  isClearing: boolean;
  canClear: boolean;
  inspectorOpen: boolean;
  onInspectorToggle: () => void;
  onDelete: () => void;
  onStop: () => void;
  onResume: () => void;
  onClear: () => void;
}

/** Publishes lifecycle actions into the owning session window's topbar. */
export function useSessionTopbarSlot({
  session,
  isDeleting,
  isStopping,
  isResuming,
  isClearing,
  canClear,
  inspectorOpen,
  onInspectorToggle,
  onDelete,
  onStop,
  onResume,
  onClear,
}: UseSessionTopbarSlotInput): void {
  const isActive = session.state === "active" || session.state === "starting";
  const canResume =
    session.attachable === true && isUserControllableSession(session) && !isSessionRunning(session);
  const showStopAction = isActive && !canResume;
  const controlsBusy = isStopping || isResuming || isDeleting;

  const primaryAction = showStopAction ? (
    <Button
      type="button"
      variant="ghost"
      size="icon-sm"
      className="size-11 focus-visible:shadow-focus-inset"
      onClick={onStop}
      disabled={controlsBusy && !isStopping}
      data-testid="stop-button"
      aria-label="Stop session"
    >
      {isStopping ? <Spinner className="size-3" /> : <Square className="size-3" />}
    </Button>
  ) : canResume ? (
    <Button
      type="button"
      variant="ghost"
      size="icon-sm"
      className="size-11 focus-visible:shadow-focus-inset"
      onClick={onResume}
      disabled={controlsBusy && !isResuming}
      data-testid="resume-button"
      aria-label="Attach session"
    >
      {isResuming ? <Spinner className="size-3" /> : <Play className="size-3" />}
    </Button>
  ) : null;

  const inspectorToggle = (
    <Button
      type="button"
      variant="ghost"
      size="icon-sm"
      aria-label={inspectorOpen ? "Close session inspector" : "Open session inspector"}
      aria-pressed={inspectorOpen}
      className={cn(
        "size-11 focus-visible:shadow-focus-inset",
        inspectorOpen ? "bg-elevated text-fg" : null
      )}
      data-state={inspectorOpen ? "open" : "closed"}
      data-testid="session-inspector-toggle"
      onClick={onInspectorToggle}
    >
      <PanelRight aria-hidden="true" className="size-3" />
    </Button>
  );

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
    status: <SessionStatusLine session={session} showState={false} />,
    actions: (
      <>
        {primaryAction}
        {inspectorToggle}
      </>
    ),
    overflow: (
      <DropdownMenu>
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
    ),
  });
}
