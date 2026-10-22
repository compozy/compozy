import { ownerDataScope } from "@/systems/workspace";
import { notifyUser } from "@/lib/user-feedback";
import type { OsOpenTarget } from "../lib/os-types";
import { loopRequestLocation } from "@/systems/loops";
import { terminalAttentionLocation } from "@/systems/terminal";

import type { DesktopOverlay } from "./use-desktop-overlays";
import type { OsAttentionRow } from "../lib/attention-model";
import { useAttentionJump } from "./use-attention-jump";
import { useOpenTarget } from "./use-open-target";
import { useOsShell } from "./use-os-shell";

const NOTIFICATION_OPEN_FAILURE = "Couldn't open this notification. Try again.";

export function useMenubarAttentionSelection(
  onOverlayOpenChange: (overlay: DesktopOverlay, open: boolean) => void
) {
  const { coordinator } = useOsShell();
  const jumpToSession = useAttentionJump();
  const openItem = useOpenTarget();
  const open = (target: OsOpenTarget) => {
    void coordinator
      .userOpen(target)
      .catch(() => notifyUser({ message: NOTIFICATION_OPEN_FAILURE, tone: "error" }));
  };

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
      // Loop rows open in place under All projects, bound to their own scope.
      case "loop-node": {
        const route = row.runId
          ? { pathname: `/loop-runs/${encodeURIComponent(row.runId)}`, search: {} }
          : { pathname: "/loop-runs", search: { nodes: row.state } };
        // A summary row names no owner: it opens the Runs view of the current scope.
        if (row.workspaceId === undefined) {
          open({ app: "loops", route });
          return;
        }
        openItem(
          { app: "loops", binding: ownerDataScope(row.workspaceId), route },
          NOTIFICATION_OPEN_FAILURE
        );
        return;
      }
      case "loop-request":
        openItem(
          {
            app: "loops",
            binding: ownerDataScope(row.workspaceId),
            route: loopRequestLocation(row),
          },
          NOTIFICATION_OPEN_FAILURE
        );
        return;
      case "terminal-input":
        // Opens in place under All projects, bound to the terminal's own scope.
        openItem(
          {
            app: "terminal",
            instanceKey: row.terminalId,
            binding: ownerDataScope(row.workspaceId),
            route: terminalAttentionLocation(row.terminalId),
          },
          "Couldn't open this notification. Try again."
        );
        return;
      case "task":
        openItem(
          {
            app: "tasks",
            binding: ownerDataScope(row.workspaceId),
            route: { pathname: `/tasks/${encodeURIComponent(row.id)}`, search: {} },
          },
          "Couldn't open this notification. Try again."
        );
    }
  };
}
