import { MonitorUp, Plus, RefreshCw } from "lucide-react";
import { useRef } from "react";
import type * as React from "react";

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
  Skeleton,
  TooltipProvider,
  cn,
} from "@compozy/ui";

import { DesktopsOverviewGrid } from "./desktops-overview-grid";

export interface DesktopOverviewWindow {
  id: string;
  title: string;
  detail?: string;
}

export interface DesktopOverviewItem {
  id: string;
  name: string;
  /** A non-interactive thumbnail projected from the authoritative layout. */
  thumbnail: React.ReactNode;
  /** Work-area width over height; the thumbnail box keeps the desk's shape. */
  aspectRatio?: number;
  windows: readonly DesktopOverviewWindow[];
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
  onMoveWindow: (windowId: string, sourceDesktopId: string, destinationDesktopId: string) => void;
  onRetry?: () => void;
  onResolveConflict?: () => void;
}

const LOADING_CARD_IDS = ["desktop-loading-1", "desktop-loading-2", "desktop-loading-3"];

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
  onMoveWindow,
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
          aria-busy={state.status === "loading"}
          data-slot="desktops-overview"
          initialFocus={initialFocusDesktopId ? initialFocusRef : undefined}
          aria-describedby={undefined}
          className={cn(
            "inset-0 h-full max-h-none w-full max-w-none translate-x-0 translate-y-0 sm:max-w-none",
            "flex flex-col gap-4.5 overflow-y-auto rounded-none bg-desk px-8 py-7 shadow-none",
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
                <p className="text-meta text-muted">
                  Each desktop keeps its own windows, tabs and layout
                </p>
              </div>
              <div className="ml-auto flex items-center gap-2">
                {ready && !empty ? (
                  <CreateDesktopButton disabled={mutationsDisabled} onClick={onCreateDesktop} />
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
                onOpenChange={onOpenChange}
                onCreateDesktop={onCreateDesktop}
                onSwitchDesktop={onSwitchDesktop}
                onRenameDesktop={onRenameDesktop}
                onReorderDesktop={onReorderDesktop}
                onDeleteDesktop={onDeleteDesktop}
                onMoveWindow={onMoveWindow}
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

function CreateDesktopButton({ disabled, onClick }: { disabled: boolean; onClick: () => void }) {
  return (
    <Button type="button" disabled={disabled} onClick={onClick}>
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
    return (
      <div className="grid grid-cols-[repeat(auto-fill,minmax(300px,1fr))] gap-5">
        {LOADING_CARD_IDS.map(id => (
          <div key={id} className="flex flex-col gap-2.5 p-2 pb-3">
            <Skeleton className="aspect-video w-full rounded-lg" />
            <Skeleton className="h-5 w-2/3" />
          </div>
        ))}
      </div>
    );
  }
  if (state.status === "error") {
    return (
      <DesktopsOverviewAlert
        actionLabel="Retry loading"
        message={state.message}
        onAction={onRetry}
        title="Couldn't load desktops"
        variant="danger"
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
  variant: "danger" | "warning";
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
