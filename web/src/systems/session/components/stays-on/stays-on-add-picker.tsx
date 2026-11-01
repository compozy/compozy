import {
  AgentPicker,
  GLOBAL_SCOPE,
  agentInstanceLimitReached,
  useAgentCreateHost,
  useAgentInstanceActions,
  useAgentInstances,
  useStaysOnAgents,
} from "@/systems/agent";

import { useSignedOutProviders } from "../../hooks/use-signed-out-providers";
import { instancesInScope } from "../../lib/stays-on-scope";

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
  const instances = useAgentInstances();
  const actions = useAgentInstanceActions();
  const catalog = useStaysOnAgents(place === GLOBAL_SCOPE ? "" : place, true);
  const signedOut = useSignedOutProviders();
  const placed = new Set(instancesInScope(instances, place).map(instance => instance.agent_name));
  const agents = (catalog.data ?? []).filter(agent => !placed.has(agent.name));
  if (catalog.isPending) return null;
  return (
    <AgentPicker
      agents={agents}
      capped={agentInstanceLimitReached(instances)}
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
