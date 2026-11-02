import { lazy, Suspense } from "react";

import type { useSessionWindowController } from "./use-session-window-controller";

const SessionClearDialog = lazy(() =>
  import("./session-window-dialogs").then(module => ({ default: module.SessionClearDialog }))
);
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

/** The lifecycle dialogs the rail and the clear command raise over the window. */
export function SessionWindowRowDialogs({
  sidebar,
  clearDialog,
  controls,
}: Pick<ReturnType<typeof useSessionWindowController>, "sidebar" | "clearDialog" | "controls">) {
  return (
    <>
      {sidebar.rowDeleteDialog.open && sidebar.rowDeleteDialog.session ? (
        <Suspense fallback={null}>
          <SessionDeleteDialog
            open
            onOpenChange={sidebar.rowDeleteDialog.onOpenChange}
            session={sidebar.rowDeleteDialog.session}
            sessions={sidebar.rowDeleteDialog.sessions}
            results={sidebar.rowDeleteDialog.results}
            onRetry={sidebar.rowDeleteDialog.onRetry}
            isDeleting={sidebar.rowDeleteDialog.isDeleting}
            onConfirm={sidebar.rowDeleteDialog.onConfirm}
          />
        </Suspense>
      ) : null}
      {sidebar.rowProcessDialog.open ? (
        <Suspense fallback={null}>
          <SessionProcessChoiceDialog {...sidebar.rowProcessDialog} />
        </Suspense>
      ) : null}
      {sidebar.rowLaterDialog.open ? (
        <Suspense fallback={null}>
          <SessionLaterDialog {...sidebar.rowLaterDialog} />
        </Suspense>
      ) : null}
      {sidebar.rowRenameDialog.open && sidebar.rowRenameDialog.session ? (
        <Suspense fallback={null}>
          <SessionRenameDialog
            open
            onOpenChange={sidebar.rowRenameDialog.onOpenChange}
            session={sidebar.rowRenameDialog.session}
            isRenaming={sidebar.rowRenameDialog.isRenaming}
            onConfirm={sidebar.rowRenameDialog.onConfirm}
          />
        </Suspense>
      ) : null}
      {clearDialog.open ? (
        <Suspense fallback={null}>
          <SessionClearDialog
            open
            onOpenChange={clearDialog.setOpen}
            isClearing={controls.isClearing}
            onConfirm={clearDialog.confirmClear}
          />
        </Suspense>
      ) : null}
    </>
  );
}
