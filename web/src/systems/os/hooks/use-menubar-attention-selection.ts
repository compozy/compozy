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

  return (row: OsAttentionRow) => {
    onOverlayOpenChange("bell", false);
    switch (row.kind) {
      case "session":
        jumpToSession({
          sessionId: row.id,
          agentName: row.agentName,
          workspaceId: row.workspaceId,
        });
        return;
      case "loop-node":
        void coordinator.userOpen({
          app: "loops",
          route: { pathname: "/loop-runs", search: { nodes: row.state } },
        });
        return;
      case "loop-request":
        void coordinator.userOpen({ app: "loops", route: loopRequestLocation(row) });
        return;
      case "terminal-input":
        void coordinator.userOpen({
          app: "terminal",
          instanceKey: row.terminalId,
          route: terminalAttentionLocation(row.terminalId),
        });
        return;
      case "task":
        void coordinator.userOpen({
          app: "tasks",
          route: { pathname: `/tasks/${encodeURIComponent(row.id)}`, search: {} },
        });
    }
  };
}
