import { useEffect, useRef } from "react";
import { useActiveWorkspace } from "@/systems/workspace";
import { notifyUser } from "@/lib/user-feedback";
import { useDesktop } from "./use-desktop";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import type { OsOpenTarget } from "../lib/os-types";
import { loopRequestLocation } from "@/systems/loops";
import { terminalAttentionLocation } from "@/systems/terminal";

import type { DesktopOverlay } from "./use-desktop-overlays";
import type { OsAttentionRow } from "../lib/attention-model";
import { useAttentionJump } from "./use-attention-jump";
import { useOsShell } from "./use-os-shell";

export function useMenubarAttentionSelection(
  onOverlayOpenChange: (overlay: DesktopOverlay, open: boolean) => void
) {
  const { coordinator } = useOsShell();
  const jumpToSession = useAttentionJump();
  const { runtimeWorkspaceId, setActiveWorkspaceId } = useActiveWorkspace();
  const commandWorkspaceId = useDesktop(state =>
    windowManagerCommandsAvailable(state) ? state.snapshot?.workspaceId : null
  );
  const pending = useRef<{ workspaceId: string; target: OsOpenTarget } | null>(null);
  const open = (target: OsOpenTarget) => {
    void coordinator
      .userOpen(target)
      .catch(() =>
        notifyUser({ message: "Couldn't open this notification. Try again.", tone: "error" })
      );
  };
  useEffect(() => {
    const next = pending.current;
    if (!next || next.workspaceId !== runtimeWorkspaceId || next.workspaceId !== commandWorkspaceId)
      return;
    pending.current = null;
    void coordinator
      .userOpen(next.target)
      .catch(() =>
        notifyUser({ message: "Couldn't open this notification. Try again.", tone: "error" })
      );
  }, [commandWorkspaceId, coordinator, runtimeWorkspaceId]);
  const openInWorkspace = (workspaceId: string | undefined, target: OsOpenTarget) => {
    pending.current = null;
    if (workspaceId && (workspaceId !== runtimeWorkspaceId || workspaceId !== commandWorkspaceId)) {
      pending.current = { workspaceId, target };
      if (workspaceId !== runtimeWorkspaceId) setActiveWorkspaceId(workspaceId);
      return;
    }
    open(target);
  };

  return (row: OsAttentionRow) => {
    onOverlayOpenChange("bell", false);
    pending.current = null;
    switch (row.kind) {
      case "session":
        jumpToSession({
          sessionId: row.id,
          agentName: row.agentName,
          workspaceId: row.workspaceId,
        });
        return;
      case "loop-node":
        openInWorkspace(row.workspaceId, {
          app: "loops",
          route: row.runId
            ? { pathname: `/loop-runs/${encodeURIComponent(row.runId)}`, search: {} }
            : { pathname: "/loop-runs", search: { nodes: row.state } },
        });
        return;
      case "loop-request":
        openInWorkspace(row.workspaceId, { app: "loops", route: loopRequestLocation(row) });
        return;
      case "terminal-input":
        openInWorkspace(row.workspaceId, {
          app: "terminal",
          instanceKey: row.terminalId,
          route: terminalAttentionLocation(row.terminalId),
        });
        return;
      case "task":
        openInWorkspace(row.workspaceId, {
          app: "tasks",
          route: { pathname: `/tasks/${encodeURIComponent(row.id)}`, search: {} },
        });
    }
  };
}
