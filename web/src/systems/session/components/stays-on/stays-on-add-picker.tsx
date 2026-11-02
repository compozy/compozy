import {
  AgentPicker,
  useAgentCreateHost,
  useAgentInstanceActions,
  useAgentInstanceCapacity,
} from "@/systems/agent";

import { useSignedOutProviders } from "../../hooks/use-signed-out-providers";
import { useUnplacedAgents } from "../../hooks/use-unplaced-agents";

/**
 * S16 from a project `+`: a popover over the same button. It mounts only while open, so nothing
 * reads the catalog until someone asks.
 */
export function StaysOnAddPicker({
  place,
  name,
  onOpenChange,
}: {
  /** A workspace id, or `@user` for Global. */
  place: string;
  name: string;
  onOpenChange: (open: boolean) => void;
}) {
  const host = useAgentCreateHost();
  const capacity = useAgentInstanceCapacity();
  const actions = useAgentInstanceActions();
  const signedOut = useSignedOutProviders();
  const { agents, pending } = useUnplacedAgents(place);
  if (pending) return null;
  return (
    <AgentPicker
      agents={agents}
      capacity={capacity}
      name={name}
      onCreateNew={() => {
        onOpenChange(false);
        host.openDialog({ place });
      }}
      onOpenChange={onOpenChange}
      onPick={agent => {
        onOpenChange(false);
        actions.add({ agent: agent.name, workspace: place });
      }}
      open
      signedOut={signedOut}
      trigger={
        <button
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 opacity-0"
          tabIndex={-1}
          type="button"
        />
      }
    />
  );
}
