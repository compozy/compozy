import { useState } from "react";

import {
  Eyebrow,
  Item,
  Popover,
  PopoverContent,
  PopoverTrigger,
  SearchInput,
  type AgentAvatarState,
} from "@compozy/ui";

import { AgentMark } from "@/systems/agent/components/agent-mark";
import {
  agentAvatarAppearance,
  agentInstanceAvatarState,
  agentInstancesByUrgency,
  type AgentInstancePayload,
} from "@/systems/agent";

const BANDS: readonly { label: string; states: readonly AgentAvatarState[] }[] = [
  { label: "Needs you", states: ["needs-you", "failed"] },
  { label: "Working", states: ["working", "starting"] },
  { label: "Resting", states: ["idle", "unknown"] },
  { label: "Paused", states: ["paused"] },
];

const SEARCH_FROM = 7;

function summary(instances: readonly AgentInstancePayload[]) {
  return BANDS.flatMap(band => {
    const count = instances.filter(instance =>
      band.states.includes(agentInstanceAvatarState(instance))
    ).length;
    return count > 0 ? [`${count} ${band.label.toLowerCase()}`] : [];
  }).join(" · ");
}

function CrewRow({
  instance,
  onOpen,
}: {
  instance: AgentInstancePayload;
  onOpen: (instance: AgentInstancePayload) => void;
}) {
  return (
    <Item
      as="button"
      className="w-full gap-3 px-2 py-1.5 text-left"
      onClick={() => onOpen(instance)}
    >
      <AgentMark
        appearance={agentAvatarAppearance(instance.resolved_appearance, instance.agent_name)}
        label={instance.label}
        provider={instance.provider}
        quietIdle
        size={32}
        state={agentInstanceAvatarState(instance)}
        surfaceClassName="ring-popover"
      />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body font-medium text-fg">{instance.label}</span>
        {instance.now ? (
          <span className="truncate text-small-body text-muted-foreground">{instance.now}</span>
        ) : null}
      </span>
    </Item>
  );
}

/** The whole crew of a scope, for when the row runs out of room. */
export function StaysOnCrewPopover({
  instances,
  hidden,
  scopeName,
  onOpen,
}: {
  /** Everyone in the row: the popover shows the whole crew, not only the overflow. */
  instances: readonly AgentInstancePayload[];
  /** How many the row couldn't fit, for the trigger's `+N`. */
  hidden: number;
  scopeName: string;
  onOpen: (instance: AgentInstancePayload) => void;
}) {
  const [query, setQuery] = useState("");
  const needle = query.trim().toLowerCase();
  const shown = agentInstancesByUrgency(instances).filter(instance =>
    instance.label.toLowerCase().includes(needle)
  );
  return (
    <Popover>
      <PopoverTrigger
        aria-label={`Show all ${instances.length} agents in ${scopeName}`}
        className="grid size-9 shrink-0 place-items-center rounded-full bg-surface-2 text-small-body font-medium text-muted-foreground tabular-nums outline-none hover:bg-hover hover:text-fg focus-visible:shadow-focus-ring"
        data-testid="stays-on-crew"
      >
        +{hidden}
      </PopoverTrigger>
      <PopoverContent align="start" className="flex w-96 flex-col gap-3 p-3">
        <div className="flex flex-col gap-0.5 px-1 pt-1">
          <span className="text-heading font-medium text-fg-strong">
            {instances.length} agents in {scopeName}
          </span>
          <span className="text-small-body text-muted-foreground">{summary(instances)}</span>
        </div>
        {instances.length >= SEARCH_FROM ? (
          <SearchInput
            aria-label="Find an agent"
            containerClassName="min-w-0"
            onChange={setQuery}
            placeholder="Find an agent…"
            value={query}
          />
        ) : null}
        <div className="flex max-h-96 flex-col gap-3 overflow-y-auto">
          {BANDS.map(band => {
            const members = shown.filter(instance =>
              band.states.includes(agentInstanceAvatarState(instance))
            );
            if (members.length === 0) return null;
            return (
              <div className="flex flex-col gap-0.5" key={band.label}>
                <Eyebrow className="px-2 pb-1 text-subtle">{band.label}</Eyebrow>
                {members.map(instance => (
                  <CrewRow instance={instance} key={instance.id} onOpen={onOpen} />
                ))}
              </div>
            );
          })}
          {shown.length === 0 ? (
            <p className="px-2 py-3 text-small-body text-muted-foreground">
              No agent goes by that name.
            </p>
          ) : null}
        </div>
      </PopoverContent>
    </Popover>
  );
}
