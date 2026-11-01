import { Eyebrow } from "@compozy/ui";

import {
  AgentMark,
  GLOBAL_SCOPE,
  agentAvatarAppearance,
  agentInstanceAvatarState,
  useAgentInstances,
  type AgentInstancePayload,
} from "@/systems/agent";

import { instancesInScope } from "../../lib/stays-on-scope";
import { StaysOnRow, type StaysOnRowProps } from "./stays-on-row";

type ScopedRowProps = Omit<StaysOnRowProps, "instances" | "scopeName">;

/** The row labeled Global, above every project. */
export function StaysOnGlobalRow(props: ScopedRowProps & { instances?: never }) {
  const instances = instancesInScope(useAgentInstances(), GLOBAL_SCOPE);
  return (
    <StaysOnRow
      {...props}
      instances={instances}
      label={<Eyebrow className="pr-1.5 text-faint">Global</Eyebrow>}
      scopeName="Global"
    />
  );
}

/** One project's row, under its name or in its lane's header. */
export function StaysOnProjectRow({
  workspaceId,
  name,
  ...props
}: ScopedRowProps & { workspaceId: string; name: string }) {
  return (
    <StaysOnRow
      {...props}
      instances={instancesInScope(useAgentInstances(), workspaceId)}
      scopeName={name}
    />
  );
}

/** A closed project still shows who wants you, by face rather than a dot. */
export function StaysOnCollapsedSignal({ workspaceId }: { workspaceId: string }) {
  const waiting = instancesInScope(useAgentInstances(), workspaceId).filter(
    instance => agentInstanceAvatarState(instance) === "needs-you"
  );
  const lead = waiting[0];
  if (!lead) return null;
  return (
    <span className="flex items-center gap-1" data-testid="stays-on-collapsed-signal">
      <AgentMark
        appearance={agentAvatarAppearance(lead.resolved_appearance, lead.agent_name)}
        label={`${lead.label} needs you`}
        size={20}
        state="needs-you"
        surfaceClassName="ring-sunken"
      />
      {waiting.length > 1 ? (
        <span className="text-micro text-muted-foreground">+{waiting.length - 1}</span>
      ) : null}
    </span>
  );
}

/** Spawned sessions stay ordinary rows and cards, with who started them. */
export function StaysOnCreatorBadge({
  creator,
  surface = "ring-sunken",
}: {
  creator: AgentInstancePayload;
  surface?: string;
}) {
  return (
    <AgentMark
      appearance={agentAvatarAppearance(creator.resolved_appearance, creator.agent_name)}
      data-testid="stays-on-creator-badge"
      label={`Started by ${creator.label}`}
      quietIdle
      size={16}
      surfaceClassName={surface}
    />
  );
}
