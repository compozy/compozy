import { ConfirmDialog, OverlayContainerContext } from "@compozy/ui";
import { type ShouldBlockFn, useBlocker } from "@tanstack/react-router";
import { createAtom } from "@xstate/store";
import { useAtom } from "@xstate/store-react";
import { type ReactNode, use, useCallback, useEffect, useRef, useState } from "react";

import { WindowScopeContext } from "@/hooks/use-window-scope";
import {
  WindowNavigationGuardContext,
  type UnsavedCloseQuestion,
  type UnsavedWindowEdits,
} from "@/hooks/use-window-navigation-guard";

export interface UseUnsavedGuardOptions {
  dirty: boolean;
  /** Entity display name interpolated into the confirmation copy. */
  entityName: string;
  /**
   * Drops the edits when the operator answers Discard changes, before the editor goes: an editor
   * that unmounts still holding edits keeps them as a draft for its next mount (#1653).
   */
  onDiscard?: () => void;
}

export interface UseUnsavedGuardResult {
  status: "idle" | "blocked";
  proceed: (() => void) | undefined;
  reset: (() => void) | undefined;
  /** Ready-to-render ConfirmDialog; consumer mounts it beside the editor. */
  confirmDialog: ReactNode;
}

const NO_CLOSE_QUESTION = createAtom<UnsavedCloseQuestion | null>(null);

/**
 * Shared unsaved-edits guard for agent editors.
 * Pins TanStack Router 1.170.17 useBlocker resolver semantics:
 * blocked.proceed allows navigation; blocked.reset stays.
 *
 * Inside the desktop shell the editor lives in a window, and the URL follows whichever window is
 * focused: only a navigation that would replace the edited resource in its own window, or leave
 * the shell, endangers the edits, so only that one asks (focusing another tab, another window's
 * navigation, Back to the editor's own route and a route naming the same page pass). The shell
 * resolves where a navigation lands. While dirty the editor also holds its window, so an operator
 * close of it (Cmd+W, the window or tab close, bulk closes) asks the same question first; one
 * question covers every window a close would take. The question shows in the editor's window
 * when it is on screen, and on the shell otherwise, never inside a hidden pane. Outside the shell
 * every navigation asks.
 */
export function useUnsavedGuard({
  dirty,
  entityName,
  onDiscard,
}: UseUnsavedGuardOptions): UseUnsavedGuardResult {
  const windowId = use(WindowScopeContext);
  const windows = use(WindowNavigationGuardContext);
  const unsavedEdits = windows?.unsavedEdits ?? null;
  // Stable while the window and the shell's answers are: the blocker registers once.
  const shouldBlockFn = useCallback<ShouldBlockFn>(
    args =>
      windowId === null ||
      windows === null ||
      // Each route's search schema is its own type; the shell compares plain search records.
      windows.replacesWindowContent(windowId, {
        pathname: args.next.pathname,
        search: Object.fromEntries(Object.entries(args.next.search)),
        routeId: args.next.routeId,
      }),
    [windowId, windows]
  );
  const blocker = useBlocker({
    shouldBlockFn,
    withResolver: true,
    disabled: !dirty,
    enableBeforeUnload: dirty,
  });

  // The shell reads the current name and discard when it asks, after this render. One object
  // for the editor's life, so the unmount cleanup reads what the last render left.
  const latest = useRef({ entityName, onDiscard, discardOnLeave: false });
  useEffect(() => {
    latest.current.entityName = entityName;
    latest.current.onDiscard = onDiscard;
  });
  useEffect(() => {
    // Edits changed after a Discard whose navigation another guard held: they are new edits.
    latest.current.discardOnLeave = false;
  }, [dirty]);
  useEffect(() => {
    const current = latest.current;
    return () => {
      if (current.discardOnLeave) current.onDiscard?.();
    };
  }, []);
  const [edits] = useState<UnsavedWindowEdits>(() => ({
    get entityName() {
      return latest.current.entityName;
    },
    discard: () => latest.current.onDiscard?.(),
  }));
  useEffect(() => {
    if (!dirty || windowId === null || unsavedEdits === null) return undefined;
    return unsavedEdits.hold(windowId, edits);
  }, [dirty, edits, unsavedEdits, windowId]);
  const closing = useAtom(unsavedEdits?.question ?? NO_CLOSE_QUESTION, question =>
    question?.presenter === edits ? question : null
  );

  // A Discard answered for a navigation drops the edits once the editor goes: another editor's
  // guard may still hold the same navigation, and then these edits stay.
  const proceed = () => {
    latest.current.discardOnLeave = true;
    blocker.proceed?.();
  };

  const reset = () => {
    blocker.reset?.();
  };

  const dialog = (
    <ConfirmDialog
      open={closing !== null || blocker.status === "blocked"}
      onOpenChange={open => {
        if (open) return;
        if (closing) closing.answer(false);
        else reset();
      }}
      tone="warning"
      title="Discard unsaved changes?"
      description={
        closing !== null && closing.windowCount > 1
          ? `Your edits in ${closing.windowCount} windows will be lost.`
          : `Your edits to ${entityName} will be lost.`
      }
      confirmLabel="Discard changes"
      cancelLabel="Keep editing"
      onConfirm={() => {
        if (closing) closing.answer(true);
        else proceed();
      }}
      confirmButtonProps={{ "data-testid": "unsaved-guard-discard" }}
      cancelButtonProps={{ "data-testid": "unsaved-guard-keep-editing" }}
      contentProps={{ "data-testid": "unsaved-guard-dialog" }}
    />
  );
  const asking = closing !== null || blocker.status === "blocked";
  const onShell =
    asking && windowId !== null && windows !== null && !windows.isWindowShown(windowId);
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
