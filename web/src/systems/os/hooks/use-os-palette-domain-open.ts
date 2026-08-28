import { useEffect, useRef } from "react";

import { notifyUser } from "@/lib/user-feedback";
import { useActiveWorkspace } from "@/systems/workspace";

import { useDesktop } from "./use-desktop";
import { useFocusedWorktreeScopeId } from "./use-worktree-scope";
import { useOsShell } from "./use-os-shell";
import { resolveAppForPath } from "../lib/app-registry";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import { applyPaletteWorktreeSelection } from "../lib/os-palette-worktree-selection";
import type { OsPaletteDomainRow } from "../lib/os-palette-domain-search";
import type { RoutingCoordinator } from "../lib/routing-coordinator";

interface PendingDomainOpen {
  readonly row: OsPaletteDomainRow;
  readonly workspaceId: string;
}

function domainOpenError(row: OsPaletteDomainRow): void {
  notifyUser({
    message: `Couldn't open ${row.label}. Try again.`,
    tone: "error",
  });
}

function openDomainRoute(coordinator: RoutingCoordinator, row: OsPaletteDomainRow): void {
  const instanceKey = resolveAppForPath(row.route.pathname)?.instanceKey;
  void coordinator
    .userOpen({
      app: row.app,
      route: row.route,
      ...(instanceKey ? { instanceKey } : {}),
    })
    .then(windowId => {
      if (windowId === null) domainOpenError(row);
    })
    .catch(() => domainOpenError(row));
}

/**
 * One landing path for concrete palette rows in both the root and pushed views.
 * Foreign workspace rows wait for the workspace window manager to become live;
 * worktree rows use the shell's canonical scope-selection action instead of a
 * dashboard search route.
 */
export function useOsPaletteDomainOpen(onDismiss: () => void): (row: OsPaletteDomainRow) => void {
  const { coordinator } = useOsShell();
  const {
    activeWorkspaceId,
    registeredWorkspaces,
    runtimeWorkspaceId,
    scope,
    setActiveWorkspaceId,
  } = useActiveWorkspace();
  const commandAvailable = useDesktop(windowManagerCommandsAvailable);
  const commandWorkspaceId = useDesktop(state => state.snapshot?.workspaceId.trim() || null);
  const worktreeScopeId = useFocusedWorktreeScopeId();
  const pendingRef = useRef<PendingDomainOpen | null>(null);

  useEffect(() => {
    const pending = pendingRef.current;
    if (pending === null) return;
    if (
      pending.workspaceId !== runtimeWorkspaceId?.trim() ||
      !commandAvailable ||
      pending.workspaceId !== commandWorkspaceId
    ) {
      return;
    }
    pendingRef.current = null;
    openDomainRoute(coordinator, pending.row);
  }, [commandAvailable, commandWorkspaceId, coordinator, runtimeWorkspaceId]);

  return (row: OsPaletteDomainRow) => {
    onDismiss();
    if (row.worktreeSelection) {
      try {
        applyPaletteWorktreeSelection({
          activeWorkspaceId,
          entry: {
            workspaceId: row.worktreeSelection.workspaceId,
            worktree: { id: row.worktreeSelection.worktreeId },
          },
          scope,
          setActiveWorkspaceId,
          worktreeScopeId,
        });
      } catch {
        domainOpenError(row);
      }
      return;
    }

    const workspaceId = row.workspaceId?.trim();
    if (!workspaceId) {
      openDomainRoute(coordinator, row);
      return;
    }
    if (!registeredWorkspaces.some(workspace => workspace.id === workspaceId)) {
      domainOpenError(row);
      return;
    }
    if (
      workspaceId !== runtimeWorkspaceId?.trim() ||
      !commandAvailable ||
      workspaceId !== commandWorkspaceId
    ) {
      pendingRef.current = { row, workspaceId };
      if (workspaceId !== runtimeWorkspaceId?.trim()) setActiveWorkspaceId(workspaceId);
      return;
    }
    openDomainRoute(coordinator, row);
  };
}
