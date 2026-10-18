import { useState } from "react";

import {
  buildSessionListSections,
  sectionsVisibleThreads,
  type SessionListSections,
} from "../lib/session-folders";
import type { SessionThreadModel } from "../lib/session-list-threads";
import type { SessionListViewModel } from "./use-session-list-view";
import { useSessionListDnd } from "./use-session-list-dnd";
import { useSessionSidebarState } from "./use-session-sidebar-state";

/** Which folder dialog is open, if any. */
export type SessionFolderDialogState =
  | { kind: "create" }
  | { kind: "rename"; folder: NonNullable<SessionListViewModel["layout"]>["folders"][number] }
  | { kind: "delete"; folder: NonNullable<SessionListViewModel["layout"]>["folders"][number] }
  | null;

/**
 * Folder presentation for one session list: the sections built from the served
 * threads, persisted folder collapse, drag wiring, and the open folder dialog.
 * The widest scope keeps rows flat, so it carries no layout.
 */
export function useSessionListFolderLayout(
  view: SessionListViewModel,
  threads: readonly SessionThreadModel[],
  filtering: boolean
) {
  const layout = view.scope === "all-workspaces" ? undefined : view.layout;
  const { collapsedFolderIds, toggleFolder } = useSessionSidebarState();
  const collapsedFolders = new Set(collapsedFolderIds);
  const sections: SessionListSections | null = layout
    ? buildSessionListSections(threads, layout.folders, { filtering, archived: view.archived })
    : null;
  const dnd = useSessionListDnd(layout, {
    manual: view.sort === "manual",
    reorderable: !filtering,
  });
  const [dialog, setDialog] = useState<SessionFolderDialogState>(null);
  return {
    layout,
    sections,
    collapsedFolders,
    toggleFolder,
    /** Threads on screen in order; bulk selection and range picks follow it. */
    visibleThreads: sections
      ? sectionsVisibleThreads(sections, collapsedFolders, filtering)
      : threads,
    dnd,
    dialog,
    setDialog,
  };
}
