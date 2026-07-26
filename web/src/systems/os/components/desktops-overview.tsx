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
  DialogDescription,
  DialogTitle,
  Empty,
  Skeleton,
  TooltipProvider,
  cn,
} from "@agh/ui";

import { DesktopsOverviewGrid } from "./desktops-overview-grid";

export interface DesktopOverviewWindow {
  id: string;
  title: string;
  detail?: string;
}

export interface DesktopOverviewItem {
  id: string;
  name: string;
  purpose: "standard" | "focus";
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
                <DialogDescription className="max-w-2xl text-small-body text-muted">
                  Switch desktops, move windows, and change their order.
                </DialogDescription>
              </div>
              {ready && !empty ? (
                <Button
                  type="button"
                  size="cta-lg"
                  disabled={mutationsDisabled}
                  onClick={onCreateDesktop}
                >
                  <Plus aria-hidden="true" />
                  Create desktop
                </Button>
              ) : null}
            </header>

            <main className="min-h-0 flex-1 overflow-y-auto pt-5">
              {state.status === "loading" ? (
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
              ) : null}

              {state.status === "error" ? (
                <Alert className="mx-auto max-w-modal-sm" variant="danger">
                  <AlertTitle>Couldn't load desktops</AlertTitle>
                  <AlertDescription>{state.message}</AlertDescription>
                  {onRetry ? (
                    <AlertActions>
                      <Button type="button" size="cta-lg" variant="neutral" onClick={onRetry}>
                        <RefreshCw aria-hidden="true" />
                        Retry loading
                      </Button>
                    </AlertActions>
                  ) : null}
                </Alert>
              ) : null}

              {state.status === "conflict" ? (
                <Alert className="mx-auto max-w-modal-sm" variant="warning">
                  <AlertTitle>Desktop layout changed</AlertTitle>
                  <AlertDescription>{state.message}</AlertDescription>
                  {onResolveConflict ? (
                    <AlertActions>
                      <Button
                        type="button"
                        size="cta-lg"
                        variant="neutral"
                        onClick={onResolveConflict}
                      >
                        <RefreshCw aria-hidden="true" />
                        Reload desktops
                      </Button>
                    </AlertActions>
                  ) : null}
                </Alert>
              ) : null}

              {empty ? (
                <Empty
                  icon={MonitorUp}
                  title="No desktops"
                  description="Create a desktop to place windows in this workspace."
                  action={
                    <Button
                      type="button"
                      size="cta-lg"
                      disabled={mutationsDisabled}
                      onClick={onCreateDesktop}
                    >
                      <Plus aria-hidden="true" />
                      Create desktop
                    </Button>
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
