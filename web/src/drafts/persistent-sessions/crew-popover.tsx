/** The whole crew of a project, for when the row runs out of room. */
import { useState } from "react";

import { Eyebrow, Item, Popover, PopoverContent, PopoverTrigger, SearchInput } from "@compozy/ui";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import type { AgentState } from "../persistent-agents-shared/agent-state";
import type { DraftInstance } from "../persistent-agents-shared/fixtures";
import { byUrgency, doingOf } from "../persistent-agents-shared/sessions-fixtures";
import { openMainSession } from "./open-main";

const BANDS: readonly { label: string; states: readonly AgentState[] }[] = [
  { label: "Needs you", states: ["needs-you", "failed"] },
  { label: "Working", states: ["working", "starting"] },
  { label: "Resting", states: ["idle", "unknown"] },
  { label: "Paused", states: ["paused"] },
];

const SEARCH_FROM = 7;

function summary(instances: readonly DraftInstance[]) {
  return BANDS.flatMap(band => {
    const count = instances.filter(instance => band.states.includes(instance.state)).length;
    return count > 0 ? [`${count} ${band.label.toLowerCase()}`] : [];
  }).join(" · ");
}

function CrewRow({ instance }: { instance: DraftInstance }) {
  return (
    <Item
      as="button"
      className="w-full gap-3 px-2 py-1.5 text-left"
      onClick={() => openMainSession(instance)}
    >
      <AgentAvatar
        appearance={instance.agent.appearance}
        label={instance.agent.label}
        provider={instance.agent.provider}
        quietIdle
        size={32}
        state={instance.state}
        surfaceClassName="ring-popover"
      />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body font-medium text-fg">{instance.agent.label}</span>
        <span className="truncate text-small-body text-muted">{doingOf(instance)}</span>
      </span>
    </Item>
  );
}

export function CrewPopover({
  instances,
  hidden,
  scopeName,
  defaultOpen = false,
}: {
  /** Everyone in the row: the popover shows the whole crew, not only the overflow. */
  instances: readonly DraftInstance[];
  /** How many the row couldn't fit, for the trigger's `+N`. */
  hidden: number;
  scopeName: string;
  defaultOpen?: boolean;
}) {
  const [query, setQuery] = useState("");
  const needle = query.trim().toLowerCase();
  const shown = byUrgency(instances).filter(instance =>
    instance.agent.label.toLowerCase().includes(needle)
  );
  return (
    <Popover defaultOpen={defaultOpen}>
      <PopoverTrigger
        aria-label={`Show all ${instances.length} agents in ${scopeName}`}
        className="grid size-9 shrink-0 place-items-center rounded-full bg-surface-2 text-small-body font-medium text-muted tabular-nums outline-none hover:bg-hover hover:text-fg focus-visible:shadow-focus-ring"
      >
        +{hidden}
      </PopoverTrigger>
      <PopoverContent align="start" className="flex w-96 flex-col gap-3 p-3">
        <div className="flex flex-col gap-0.5 px-1 pt-1">
          <span className="text-heading font-medium text-fg-strong">
            {instances.length} agents in {scopeName}
          </span>
          <span className="text-small-body text-muted">{summary(instances)}</span>
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
            const members = shown.filter(instance => band.states.includes(instance.state));
            if (members.length === 0) return null;
            return (
              <div className="flex flex-col gap-0.5" key={band.label}>
                <Eyebrow className="px-2 pb-1 text-subtle">{band.label}</Eyebrow>
                {members.map(instance => (
                  <CrewRow instance={instance} key={instance.id} />
                ))}
              </div>
            );
          })}
          {shown.length === 0 ? (
            <p className="px-2 py-3 text-small-body text-muted">No agent goes by that name.</p>
          ) : null}
        </div>
      </PopoverContent>
    </Popover>
  );
}
