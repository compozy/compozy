import { useEffect, useRef } from "react";

import { notifyUser } from "@/lib/user-feedback";
import { focusDestination, useActiveWorkspace } from "@/systems/workspace";

import { openTarget, runOpen, type OpenTarget, type PendingOpen } from "../lib/open-target";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

export type OpenTargetFn = (target: OpenTarget, failureMessage?: string) => void;

const DEFAULT_FAILURE = "Couldn't open this item. Try again.";

/**
 * Binds `openTarget` to the shell: the live command partition, the menubar
 * destination, and one held open that runs once its desktop is live. A newer
 * open replaces a held one.
 */
export function useOpenTarget(): OpenTargetFn {
  const { coordinator } = useOsShell();
  const { dataScope, desktopWorkspaceId } = useActiveWorkspace();
  const commandPartition = useDesktop(state =>
    windowManagerCommandsAvailable(state) ? state.snapshot?.workspaceId.trim() || null : null
  );
  const pendingRef = useRef<(PendingOpen & { failure: string }) | null>(null);

  useEffect(() => {
    const pending = pendingRef.current;
    if (
      pending === null ||
      commandPartition !== pending.partition ||
      desktopWorkspaceId !== pending.partition
    ) {
      return;
    }
    pendingRef.current = null;
    void runOpen(coordinator, pending).catch(() =>
      notifyUser({ message: pending.failure, tone: "error" })
    );
  }, [commandPartition, coordinator, desktopWorkspaceId]);

  return (target, failureMessage = DEFAULT_FAILURE) => {
    pendingRef.current = null;
    void openTarget(target, {
      coordinator,
      dataScope,
      commandPartition,
      focusDestination,
      defer: pending => {
        pendingRef.current = { ...pending, failure: failureMessage };
      },
    }).catch(() => notifyUser({ message: failureMessage, tone: "error" }));
  };
}
