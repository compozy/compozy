import { ConfirmDialog, OverlayContainerContext } from "@compozy/ui";
import { type ShouldBlockFn, useBlocker } from "@tanstack/react-router";
import { type ReactNode, use, useCallback } from "react";

import { WindowScopeContext } from "@/hooks/use-window-scope";
import { WindowNavigationGuardContext } from "@/hooks/use-window-navigation-guard";

export interface UseUnsavedGuardOptions {
  dirty: boolean;
  /** Entity display name interpolated into the confirmation copy. */
  entityName: string;
}

export interface UseUnsavedGuardResult {
  status: "idle" | "blocked";
  proceed: (() => void) | undefined;
  reset: (() => void) | undefined;
  /** Ready-to-render ConfirmDialog; consumer mounts it beside the editor. */
  confirmDialog: ReactNode;
}

/**
 * Shared unsaved navigation guard for agent editors.
 * Pins TanStack Router 1.170.17 useBlocker resolver semantics:
 * blocked.proceed allows navigation; blocked.reset stays.
 *
 * Inside the desktop shell the editor lives in a window, and the URL follows whichever window is
 * focused: only a navigation that would replace the editor in its own window endangers the
 * edits, so only that one asks (focusing another tab, another window's navigation and Back to
 * the editor's own route pass). The shell resolves where a navigation lands. The question shows
 * in the editor's window when it is on screen, and on the shell otherwise, never inside a hidden
 * pane. Outside the shell every navigation asks.
 */
export function useUnsavedGuard({
  dirty,
  entityName,
}: UseUnsavedGuardOptions): UseUnsavedGuardResult {
  const windowId = use(WindowScopeContext);
  const windows = use(WindowNavigationGuardContext);
  // Stable while the window and the shell's answers are: the blocker registers once.
  const shouldBlockFn = useCallback<ShouldBlockFn>(
    args =>
      windowId === null ||
      windows === null ||
      // Each route's search schema is its own type; the shell compares plain search records.
      windows.replacesWindowContent(windowId, {
        pathname: args.next.pathname,
        search: Object.fromEntries(Object.entries(args.next.search)),
      }),
    [windowId, windows]
  );
  const blocker = useBlocker({
    shouldBlockFn,
    withResolver: true,
    disabled: !dirty,
    enableBeforeUnload: dirty,
  });

  const proceed = () => {
    blocker.proceed?.();
  };

  const reset = () => {
    blocker.reset?.();
  };

  const dialog = (
    <ConfirmDialog
      open={blocker.status === "blocked"}
      onOpenChange={open => {
        if (!open) reset();
      }}
      tone="warning"
      title="Discard unsaved changes?"
      description={`Your edits to ${entityName} will be lost.`}
      confirmLabel="Discard changes"
      cancelLabel="Keep editing"
      onConfirm={proceed}
      confirmButtonProps={{ "data-testid": "unsaved-guard-discard" }}
      cancelButtonProps={{ "data-testid": "unsaved-guard-keep-editing" }}
      contentProps={{ "data-testid": "unsaved-guard-dialog" }}
    />
  );
  const onShell =
    blocker.status === "blocked" &&
    windowId !== null &&
    windows !== null &&
    !windows.isWindowShown(windowId);
  const confirmDialog = onShell ? (
    <OverlayContainerContext value={null}>{dialog}</OverlayContainerContext>
  ) : (
    dialog
  );

  return {
    status: blocker.status,
    proceed: blocker.status === "blocked" ? proceed : undefined,
    reset: blocker.status === "blocked" ? reset : undefined,
    confirmDialog,
  };
}
