import { useSessionLifecycleActions, type SessionListViewModel } from "@/systems/session";
import {
  useWorkspaceSetupContent,
  WorkspaceSetupDialog,
  type WorkspaceSetupDefaultsModel,
} from "@/systems/workspace";

import { OsSessionsDialogHost } from "./sessions-modal";

export interface DesktopSessionDialogsProps {
  workspaceId: string | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  disconnected: boolean;
  /** Breadth, order, catalog, and widened workspace groups — owned by the shell. */
  view: SessionListViewModel;
  onNewSession: () => void;
}

/**
 * The shell's sessions catalog host. The lifecycle actions live here so the
 * modal and the delete/rename dialogs it opens share one owner.
 */
export function DesktopSessionDialogs({
  workspaceId,
  open,
  onOpenChange,
  disconnected,
  view,
  onNewSession,
}: DesktopSessionDialogsProps) {
  const lifecycle = useSessionLifecycleActions({ workspaceId });
  return (
    <OsSessionsDialogHost
      open={open}
      onOpenChange={onOpenChange}
      disconnected={disconnected}
      view={view}
      currentWorkspaceId={workspaceId}
      onNewSession={onNewSession}
      lifecycle={lifecycle}
    />
  );
}

/**
 * Mirrors `WorkspaceSetupDialogBoundary`: the domain hook runs in exactly one
 * place and only while the dialog is mounted, so a closed dialog holds no
 * mutation state.
 */
export function WorkspaceSetupDialogBoundary({
  defaults,
  onOpenChange,
  onWorkspaceResolved,
  open,
}: {
  defaults: WorkspaceSetupDefaultsModel;
  onOpenChange: (open: boolean) => void;
  onWorkspaceResolved: (workspaceId: string) => void;
  open: boolean;
}) {
  const setup = useWorkspaceSetupContent({
    onWorkspaceResolved,
    onSuccessClose: () => onOpenChange(false),
  });
  return (
    <WorkspaceSetupDialog model={{ defaults, setup }} onOpenChange={onOpenChange} open={open} />
  );
}
