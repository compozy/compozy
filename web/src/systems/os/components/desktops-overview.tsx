import { MonitorUp, Plus, RefreshCw } from "@compozy/ui/icons";
import { useRef, type PointerEvent } from "react";

import {
  Alert,
  AlertActions,
  AlertDescription,
  AlertTitle,
  Button,
  Dialog,
  DialogContent,
  DialogTitle,
  Empty,
  OverlayContainerContext,
  TooltipProvider,
  cn,
} from "@compozy/ui";

import type { SessionPayload } from "@/systems/session";

import type { OsWindow } from "../lib/os-types";
import type { LayoutNode, TabId, WindowId } from "../lib/window-manager-types";
import { DesktopsOverviewGrid, DesktopsOverviewSkeleton } from "./desktops-overview-grid";

export interface DesktopOverviewTab {
  id: TabId;
  /** The member that names the tab and takes focus when it is chosen. */
  leadWindowId: WindowId;
  /** Members in reading order, lead first. */
  members: readonly OsWindow[];
}

export interface DesktopOverviewItem {
  id: string;
  name: string;
  /** The desktop's tabs in bar order. */
  tabs: readonly DesktopOverviewTab[];
  /** The tab this client shows on the desktop. */
  activeTabId: TabId | null;
  /** That tab's split tree, drawn as the card's thumbnail. */
  activeRoot: LayoutNode | null;
  /** A window on this desktop needs you. */
  needsYou?: boolean;
  /** The live switch chord for this position ("⌃1"), when bound. */
  switchShortcut?: string | null;
}

export type DesktopsOverviewState =
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "conflict"; message: string }
  | {
      status: "ready";
      desktops: readonly DesktopOverviewItem[];
      activeDesktopId: string | null;
    };

export interface DesktopOverviewFocusSegment {
  readonly direction: "earlier" | "later";
  readonly hiddenDesktopIds: readonly string[];
}

export interface DesktopsOverviewProps {
  open: boolean;
  /**
   * The desk's overlay host: the overview covers the desk only, leaving the
   * topbar and rail in place. Omitted, it covers the viewport.
   */
  container?: HTMLElement | null;
  state: DesktopsOverviewState;
  initialFocusSegment?: DesktopOverviewFocusSegment | null;
  busy?: boolean;
  canMutate?: boolean;
  onOpenChange: (open: boolean) => void;
  onCreateDesktop: () => void;
  onSwitchDesktop: (desktopId: string) => void;
  onRenameDesktop: (desktopId: string, name: string) => void;
  onReorderDesktop: (desktopId: string, targetIndex: number) => void;
  onDeleteDesktop: (desktopId: string, transferToDesktopId: string | null) => void;
  /** "Move a tab ▸": the tab of `leadWindowId`, split included, joins the destination's bar. */
  onMoveTab: (leadWindowId: string, destinationDesktopId: string) => void;
  /** Choosing a tab: switch to its desktop and activate it; the overview then closes. */
  onChooseTab: (desktopId: string, windowId: string) => void;
  /** A press on a tab row; it becomes a tab drag toward another card. */
  onTabPointerDown?: (tabId: TabId, event: PointerEvent<HTMLElement>) => void;
  /** The card a dragged tab would move to on release. */
  dropTargetDesktopId?: string | null;
  /** Live session facts for tab titles and signals. */
  sessionOf?: (win: OsWindow) => SessionPayload | undefined;
  onRetry?: () => void;
  onResolveConflict?: () => void;
}

function nearestHiddenDesktopId(
  segment: DesktopOverviewFocusSegment | null | undefined
): string | null {
  if (!segment) return null;
  if (segment.direction === "later") return segment.hiddenDesktopIds[0] ?? null;
  return segment.hiddenDesktopIds[segment.hiddenDesktopIds.length - 1] ?? null;
}

/** On-demand, daemon-backed management surface for persistent workspace desktops. */
export function DesktopsOverview({
  open,
  container = null,
  state,
  initialFocusSegment,
  busy = false,
  canMutate = true,
  onOpenChange,
  onCreateDesktop,
  onSwitchDesktop,
  onRenameDesktop,
  onReorderDesktop,
  onDeleteDesktop,
  onMoveTab,
  onChooseTab,
  onTabPointerDown,
  dropTargetDesktopId = null,
  sessionOf = noSession,
  onRetry,
  onResolveConflict,
}: DesktopsOverviewProps) {
  const initialFocusRef = useRef<HTMLButtonElement | null>(null);
  const ready = state.status === "ready" ? state : null;
  const empty = ready?.desktops.length === 0;
  const requestedFocusDesktopId =
    nearestHiddenDesktopId(initialFocusSegment) ?? ready?.activeDesktopId ?? null;
  const initialFocusDesktopId =
    ready?.desktops.some(desktop => desktop.id === requestedFocusDesktopId) === true
      ? requestedFocusDesktopId
      : null;
  const mutationsDisabled = busy || !canMutate;

  return (
    <OverlayContainerContext value={container}>
      <Dialog
        open={open}
        modal
        disablePointerDismissal={false}
        onOpenChange={(next, details) => {
          // The All desktops button toggles the overview itself; an outside
          // press on it must not close first and let the click reopen it.
          if (!next && isDesktopsToggle(details.event?.target)) return;
          onOpenChange(next);
        }}
      >
        <DialogContent
          unframed
          showCloseButton={false}
          data-slot="desktops-overview"
          initialFocus={initialFocusDesktopId ? initialFocusRef : undefined}
          aria-describedby={undefined}
          className={cn(
            "inset-0 h-full max-h-none w-full max-w-none translate-x-0 translate-y-0 sm:max-w-none",
            "flex flex-col gap-5 overflow-y-auto rounded-none bg-overlay-scrim px-8 py-7 shadow-none backdrop-blur-md",
            "origin-center transition-[opacity,scale] duration-shell-base ease-spring",
            "data-starting-style:scale-98.5 data-starting-style:opacity-0 motion-reduce:transition-none"
          )}
          style={{ maxHeight: "none" }}
        >
          <TooltipProvider>
            <header className="flex items-center gap-3">
              <div className="flex min-w-0 flex-col">
                <DialogTitle className="text-heading font-medium tracking-tight text-fg">
                  Desktops
                </DialogTitle>
                <p className="text-meta text-fg-2">Each desktop keeps its own tabs</p>
              </div>
              <div className="ml-auto flex items-center gap-2">
                {ready && !empty ? (
                  <CreateDesktopButton
                    disabled={mutationsDisabled}
                    variant="secondary"
                    onClick={onCreateDesktop}
                  />
                ) : null}
                <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
                  Done
                </Button>
              </div>
            </header>

            <DesktopsOverviewStatus
              state={state}
              onRetry={onRetry}
              onResolveConflict={onResolveConflict}
            />

            {empty ? (
              <Empty
                icon={MonitorUp}
                title="No desktops"
                description="Add a desktop to arrange windows."
                action={
                  <CreateDesktopButton disabled={mutationsDisabled} onClick={onCreateDesktop} />
                }
              />
            ) : null}

            {ready && !empty ? (
              <DesktopsOverviewGrid
                state={ready}
                busy={mutationsDisabled}
                initialFocusDesktopId={initialFocusDesktopId}
                initialFocusRef={initialFocusRef}
                sessionOf={sessionOf}
                dropTargetDesktopId={dropTargetDesktopId}
                onOpenChange={onOpenChange}
                onSwitchDesktop={onSwitchDesktop}
                onRenameDesktop={onRenameDesktop}
                onReorderDesktop={onReorderDesktop}
                onDeleteDesktop={onDeleteDesktop}
                onMoveTab={onMoveTab}
                onChooseTab={onChooseTab}
                onTabPointerDown={onTabPointerDown}
              />
            ) : null}
          </TooltipProvider>
        </DialogContent>
      </Dialog>
    </OverlayContainerContext>
  );
}

function isDesktopsToggle(target: EventTarget | null | undefined): boolean {
  return target instanceof Element && target.closest('[data-slot="os-menubar-desktops"]') !== null;
}

function noSession(): undefined {
  return undefined;
}

function CreateDesktopButton({
  disabled,
  variant = "default",
  onClick,
}: {
  disabled: boolean;
  variant?: "default" | "secondary";
  onClick: () => void;
}) {
  return (
    <Button type="button" variant={variant} disabled={disabled} onClick={onClick}>
      <Plus aria-hidden="true" data-icon="inline-start" />
      New desktop
    </Button>
  );
}

interface DesktopsOverviewStatusProps {
  state: DesktopsOverviewState;
  onRetry?: () => void;
  onResolveConflict?: () => void;
}

/** Loading skeletons, or the load-failure / stale-layout alert; nothing once ready. */
function DesktopsOverviewStatus({
  state,
  onRetry,
  onResolveConflict,
}: DesktopsOverviewStatusProps) {
  if (state.status === "loading") {
    return <DesktopsOverviewSkeleton />;
  }
  if (state.status === "error") {
    return (
      <DesktopsOverviewAlert
        actionLabel="Retry loading"
        message={state.message}
        onAction={onRetry}
        title="Couldn't load desktops"
        variant="destructive"
      />
    );
  }
  if (state.status === "conflict") {
    return (
      <DesktopsOverviewAlert
        actionLabel="Reload desktops"
        message={state.message}
        onAction={onResolveConflict}
        title="Desktop layout changed"
        variant="warning"
      />
    );
  }
  return null;
}

interface DesktopsOverviewAlertProps {
  variant: "destructive" | "warning";
  title: string;
  message: string;
  actionLabel: string;
  onAction?: () => void;
}

function DesktopsOverviewAlert({
  variant,
  title,
  message,
  actionLabel,
  onAction,
}: DesktopsOverviewAlertProps) {
  return (
    <Alert className="mx-auto max-w-(--width-modal-sm)" variant={variant}>
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>{message}</AlertDescription>
      {onAction ? (
        <AlertActions>
          <Button type="button" size="cta-lg" variant="neutral" onClick={onAction}>
            <RefreshCw aria-hidden="true" />
            {actionLabel}
          </Button>
        </AlertActions>
      ) : null}
    </Alert>
  );
}
