import { useNavigate } from "@tanstack/react-router";

import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import { LoopEditor } from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";

export function LoopEditorLocation({ name }: { name: string }) {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const navigate = useNavigate();
  const openLoops = () => void navigate({ to: "/loops" });
  const openLoop = () => void navigate({ to: "/loops/$name", params: { name } });
  return (
    <LoopEditor
      workspaceId={runtimeWorkspaceId ?? ""}
      name={name}
      liveDataEnabled={liveDataEnabled}
      topbarIdentity={{
        onBack: openLoop,
        crumbs: [
          { id: "loops", label: "Loops", onSelect: openLoops },
          { id: "loop", label: name, onSelect: openLoop },
        ],
        crumb: "Editor",
      }}
      // Publish → continue to the run form for the freshly published definition.
      onPublished={loop => void navigate({ to: "/loops/$name/run", params: { name: loop.name } })}
    />
  );
}
