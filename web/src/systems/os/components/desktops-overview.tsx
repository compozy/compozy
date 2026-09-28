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
  windows: readonly DesktopOverviewWindow[];
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
  const requestedFocusDesktopId = nearestHiddenDesktopId(initialFocusSegment);
  const initialFocusDesktopId =
    ready?.desktops.some(desktop => desktop.id === requestedFocusDesktopId) === true
      ? requestedFocusDesktopId
      : null;
  const mutationsDisabled = busy || !canMutate;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        unframed
        aria-busy={state.status === "loading"}
        data-slot="desktops-overview"
        initialFocus={initialFocusDesktopId ? initialFocusRef : undefined}
        aria-describedby={undefined}
        className={cn(
          "top-0 left-0 h-full w-full max-w-none translate-x-0 translate-y-0 rounded-none sm:max-w-none",
          "overflow-hidden bg-transparent shadow-none backdrop-blur-shell-scrim"
        )}
      >
        <TooltipProvider>
          <div className="mx-auto flex h-full w-full max-w-(--width-modal-xl) flex-col px-4 pt-12 sm:px-6">
            <header className="flex items-start justify-between gap-4 border-b border-line pb-4">
              <div className="flex min-w-0 flex-col gap-1">
                <DialogTitle className="text-compact-h1 font-semibold text-fg-strong">
                  Desktops
                </DialogTitle>
              </div>
              {ready && !empty ? (
                <CreateDesktopButton disabled={mutationsDisabled} onClick={onCreateDesktop} />
              ) : null}
            </header>

            <main className="min-h-0 flex-1 overflow-y-auto pt-5">
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
                  onSwitchDesktop={onSwitchDesktop}
                  onRenameDesktop={onRenameDesktop}
                  onReorderDesktop={onReorderDesktop}
                  onDeleteDesktop={onDeleteDesktop}
                  onMoveWindow={onMoveWindow}
                />
              ) : null}
            </main>
          </div>
        </TooltipProvider>
      </DialogContent>
    </Dialog>
  );
}

function CreateDesktopButton({ disabled, onClick }: { disabled: boolean; onClick: () => void }) {
  return (
    <Button type="button" size="cta-lg" disabled={disabled} onClick={onClick}>
      <Plus aria-hidden="true" />
      Create desktop
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
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
        {LOADING_CARD_IDS.map(id => (
          <div
            key={id}
            className="flex flex-col gap-3 rounded-lg border border-line bg-canvas-soft p-3"
          >
            <Skeleton className="h-5 w-2/3" />
            <Skeleton className="h-workspace-thumb w-full" />
            <Skeleton className="h-8 w-full" />
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
    <Alert className="mx-auto max-w-modal-sm" variant={variant}>
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
