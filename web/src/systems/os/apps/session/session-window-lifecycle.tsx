import { lazy, Suspense, type ReactNode } from "react";

import { useSessionListLifecycleActions } from "../../hooks/use-session-list-lifecycle-actions";
import {
  SessionWindowLifecycleContext,
  type SessionWindowLifecycle,
} from "./session-window-lifecycle-context";

const SessionProcessChoiceDialog = lazy(() =>
  import("@/systems/session").then(module => ({
    default: module.SessionProcessChoiceDialog,
  }))
);
const SessionDeleteDialog = lazy(() =>
  import("@/systems/session/components/session-delete-dialog").then(module => ({
    default: module.SessionDeleteDialog,
  }))
);
const SessionLaterDialog = lazy(() =>
  import("@/systems/session/components/session-later-dialog").then(module => ({
    default: module.SessionLaterDialog,
  }))
);
const SessionRenameDialog = lazy(() =>
  import("@/systems/session/components/session-rename-dialog").then(module => ({
    default: module.SessionRenameDialog,
  }))
);

/**
 * One lifecycle instance per session window, mounted above the content/empty branch.
 * The confirmations live here, not in the body: a delete removes the window's own session
 * before its request settles, so a body-hosted dialog would be unmounted mid-flight (and a
 * batch delete would orphan its dialog between per-target mutations). The window's session
 * view keeps rendering its delete target while that request runs, so the confirmation and the
 * conversation it covers never replay an open/close transition.
 */
export function SessionWindowLifecycleHost({
  windowId,
  children,
}: {
  windowId: string;
  children: ReactNode;
}) {
  const lifecycle = useSessionListLifecycleActions({ windowId });
  return (
    <SessionWindowLifecycleContext value={lifecycle}>
      {children}
      <SessionWindowLifecycleDialogs lifecycle={lifecycle} />
    </SessionWindowLifecycleContext>
  );
}

/** The rail's single or batch confirmations, owned by the window and never by the body. */
function SessionWindowLifecycleDialogs({ lifecycle }: { lifecycle: SessionWindowLifecycle }) {
  const { deleteDialog, processDialog, laterDialog, renameDialog } = lifecycle;
  return (
    <>
      {deleteDialog.open && deleteDialog.session ? (
        <Suspense fallback={null}>
          <SessionDeleteDialog
            open
            onOpenChange={deleteDialog.onOpenChange}
            session={deleteDialog.session}
            sessions={deleteDialog.sessions}
            results={deleteDialog.results}
            onRetry={deleteDialog.onRetry}
            isDeleting={deleteDialog.isDeleting}
            childSessions={deleteDialog.childSessions}
            onConfirm={deleteDialog.onConfirm}
          />
        </Suspense>
      ) : null}
      {processDialog.open ? (
        <Suspense fallback={null}>
          <SessionProcessChoiceDialog {...processDialog} />
        </Suspense>
      ) : null}
      {laterDialog.open ? (
        <Suspense fallback={null}>
          <SessionLaterDialog {...laterDialog} />
        </Suspense>
      ) : null}
      {renameDialog.open && renameDialog.session ? (
        <Suspense fallback={null}>
          <SessionRenameDialog
            open
            onOpenChange={renameDialog.onOpenChange}
            session={renameDialog.session}
            isRenaming={renameDialog.isRenaming}
            onConfirm={renameDialog.onConfirm}
          />
        </Suspense>
      ) : null}
    </>
  );
}
