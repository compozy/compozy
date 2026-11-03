import { useState } from "react";

import {
  AgentPicker,
  AgentScopePicker,
  useAgentCreateHost,
  useAgentInstanceActions,
  useAgentInstanceCapacity,
} from "@/systems/agent";

import { useSignedOutProviders } from "../../hooks/use-signed-out-providers";
import { useStaysOnPlaces } from "../../hooks/use-stays-on-places";
import { useUnplacedAgents } from "../../hooks/use-unplaced-agents";

/** Where an added stays-on agent lives. */
export interface SessionListStaysOnTarget {
  /** A workspace id, or `@user` for Global. */
  place: string;
  name: string;
}

interface StaysOnAddPickerProps {
  /** The list's one concrete scope, or `null` when it spans several: the operator picks one first. */
  target: SessionListStaysOnTarget | null;
  onOpenChange: (open: boolean) => void;
}

/** Invisible anchor: the picker opens over the `+` that mounted it. */
const ANCHOR = (
  <button
    aria-hidden="true"
    className="pointer-events-none absolute inset-0 opacity-0"
    tabIndex={-1}
    type="button"
  />
);

/**
 * S16 from a `+`: a popover over the same button. It mounts only while open, so nothing
 * reads the catalog until someone asks.
 */
export function StaysOnAddPicker({ target, onOpenChange }: StaysOnAddPickerProps) {
  const [picked, setPicked] = useState(target);
  if (picked === null) return <StaysOnPlaceStep onOpenChange={onOpenChange} onPick={setPicked} />;
  return <StaysOnAgentStep onOpenChange={onOpenChange} target={picked} />;
}

/** Where the agent lives, for a list showing several projects. */
function StaysOnPlaceStep({
  onPick,
  onOpenChange,
}: {
  onPick: (target: SessionListStaysOnTarget) => void;
  onOpenChange: (open: boolean) => void;
}) {
  const { places, preselected, pending } = useStaysOnPlaces();
  const capacity = useAgentInstanceCapacity();
  // The list opens on the preselected row once, so it waits for the projects that settle it.
  if (pending) return null;
  return (
    <AgentScopePicker
      capacity={capacity}
      onOpenChange={onOpenChange}
      onPick={place =>
        onPick({ place, name: places.find(option => option.id === place)?.name ?? place })
      }
      open
      scopes={places}
      trigger={ANCHOR}
      value={preselected}
    />
  );
}

/** Which agent to add to the settled place. */
function StaysOnAgentStep({
  target,
  onOpenChange,
}: {
  target: SessionListStaysOnTarget;
  onOpenChange: (open: boolean) => void;
}) {
  const { place, name } = target;
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
      trigger={ANCHOR}
    />
  );
}
