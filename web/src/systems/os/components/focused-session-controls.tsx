import { useLayoutEffect, type RefObject } from "react";

import {
  useSessionListView,
  useSessionSidebarState,
  type SessionCreateOpenOptions,
  type SessionListViewModel,
} from "@/systems/session";
import type { DataScope } from "@/systems/workspace";

import { DesktopSessionDialogs } from "./desktop-shell-dialogs";

/** The focused window's sessions list and rail, as shell gestures (⌘B, cycling) read them. */
export interface FocusedSessionControls {
  listView: SessionListViewModel;
  sidebar: ReturnType<typeof useSessionSidebarState>;
}

export interface FocusedSessionControlsHostProps {
  /** Where the shell body's gestures read the current controls when they run. */
  controlsRef: RefObject<FocusedSessionControls | null>;
  dataScope: DataScope | null;
  worktreeId: string | undefined;
  worktreeResolved: boolean;
  workspaceId: string | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  attentionDisconnected: boolean;
  onNewSession: (options?: SessionCreateOpenOptions) => void;
}

/**
 * The Sessions dialog with the sessions list and rail state of the focused window. Both are kept
 * per window, under the window's own scope (`FocusedWindowScope` above), so this leaf re-renders
 * when focus moves; the shell body does not; its gestures read `controlsRef` when they run.
 */
export function FocusedSessionControlsHost({
  controlsRef,
  dataScope,
  worktreeId,
  worktreeResolved,
  workspaceId,
  open,
  onOpenChange,
  attentionDisconnected,
  onNewSession,
}: FocusedSessionControlsHostProps) {
  // Scope and order are the operator's, persisted by the daemon; the modal
  // renders them rather than fetching its own.
  const listView = useSessionListView({
    scope: dataScope,
    worktreeId,
    enabled: worktreeResolved,
    pending: !worktreeResolved,
  });
  const sidebar = useSessionSidebarState();
  useLayoutEffect(() => {
    controlsRef.current = { listView, sidebar };
  });
  return (
    <DesktopSessionDialogs
      workspaceId={workspaceId}
      open={open}
      onOpenChange={onOpenChange}
      disconnected={attentionDisconnected || Boolean(listView.catalog?.failed)}
      view={listView}
      onNewSession={onNewSession}
    />
  );
}
