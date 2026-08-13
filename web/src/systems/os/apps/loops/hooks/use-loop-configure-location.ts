import { useNavigate } from "@tanstack/react-router";

import { useTopbarSlot } from "@compozy/ui";
import { useLoop, useLoopConfig } from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";
import { useCurrentWindowLiveDataEnabled } from "../../../hooks/use-window-live-data-enabled";

export function useLoopConfigureLocation(name: string) {
  const navigate = useNavigate();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = runtimeWorkspaceId ?? "";
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const queryEnabled = workspaceId !== "" && liveDataEnabled;
  const loopQuery = useLoop(workspaceId, name, queryEnabled);
  const configQuery = useLoopConfig(workspaceId, name, queryEnabled);
  const openLoops = () => void navigate({ to: "/loops" });
  const close = () => void navigate({ to: "/loops/$name", params: { name } });

  useTopbarSlot({
    onBack: close,
    crumbs: [
      { id: "loops", label: "Loops", onSelect: openLoops },
      { id: "loop", label: name, onSelect: close },
    ],
    crumb: "Configure",
  });

  return {
    close,
    configQuery,
    loopQuery,
    openEditor: () => void navigate({ to: "/loops/$name/editor", params: { name } }),
    workspaceId,
  };
}
