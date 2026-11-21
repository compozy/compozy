import { lazy, Suspense } from "react";

import type { useSessionWindowController } from "./use-session-window-controller";
import type { SessionWindowSidebarModel } from "./use-session-window-sidebar";

const SessionClearDialog = lazy(() =>
  import("./session-window-dialogs").then(module => ({ default: module.SessionClearDialog }))
);
const SessionProcessChoiceDialog = lazy(() =>
  import("@/systems/session").then(module => ({
    default: module.SessionProcessChoiceDialog,
  }))
);
const SessionLaterDialog = lazy(() =>
  import("@/systems/session").then(module => ({
    default: module.SessionLaterDialog,
  }))
);
const SessionRenameDialog = lazy(() =>
  import("@/systems/session").then(module => ({
    default: module.SessionRenameDialog,
  }))
);

/** The lifecycle dialogs the rail's rows raise over the window; they outlive the session pane. */
export function SessionWindowRowDialogs({ sidebar }: { sidebar: SessionWindowSidebarModel }) {
  return (
    <>
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
    </>
  );
}

/** The clear command's confirmation over the session pane. */
export function SessionWindowClearDialog({
  clearDialog,
  controls,
}: Pick<ReturnType<typeof useSessionWindowController>, "clearDialog" | "controls">) {
  return (
    <>
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
