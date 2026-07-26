import { Fragment, useState } from "react";
import { Search } from "lucide-react";

import { Eyebrow, Input } from "@agh/ui";

import { listEventGroups, type EventDef, type EventFamily } from "../../lib/trigger-catalog";
import type { EventSelection } from "../../lib/trigger-event-id";
import type { SubConfigValues } from "../../lib/trigger-sub-config";
import { EventCard } from "./event-card";
import { EventSubConfig } from "./event-sub-config";

interface EventCatalogProps {
  disabledCatalogIds?: ReadonlySet<string>;
  selection: EventSelection;
  subConfigValues: SubConfigValues;
  onSelectEvent: (catalogId: string) => void;
  onSubConfigChange: (patch: Partial<SubConfigValues>) => void;
}

function displayIdFor(event: EventDef): string {
  if (event.id === "hook.completed") return "hook.<name>.completed";
  if (event.id === "ext") return "ext.<ext>.<event>";
  return event.id;
}

function needsSubConfig(family: EventFamily): boolean {
  return family === "hook" || family === "ext" || family === "webhook";
}

/** "When" step: searchable event catalog with inline per-event configuration. */
export function EventCatalog({
  disabledCatalogIds,
  selection,
  subConfigValues,
  onSelectEvent,
  onSubConfigChange,
}: EventCatalogProps) {
  const [query, setQuery] = useState("");
  const groups = listEventGroups(query);

  return (
    <div>
      <div className="relative mb-3">
        <Search
          aria-hidden="true"
          className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-subtle"
        />
        <Input
          aria-label="Search events"
          className="pl-8"
          onChange={event => setQuery(event.target.value)}
          placeholder="Search events… session, memory, webhook"
          value={query}
        />
      </div>

      {groups.length === 0 ? (
        <div className="rounded-md border border-dashed border-line-soft bg-canvas-tint px-3 py-2.5 text-form-label text-subtle">
          No events match “{query}”.
        </div>
      ) : (
        groups.map(({ group, events }) => (
          <div className="mb-3" key={group}>
            <Eyebrow className="mb-1.5 ml-0.5 block text-faint">{group}</Eyebrow>
            <div className="space-y-1.5">
              {events.map(event => {
                const selected = event.id === selection.catalogId;
                return (
                  <Fragment key={event.id}>
                    <EventCard
                      catalogId={event.id}
                      description={event.description}
                      disabled={disabledCatalogIds?.has(event.id) ?? false}
                      displayId={displayIdFor(event)}
                      icon={event.icon}
                      label={event.label}
                      onSelect={() => onSelectEvent(event.id)}
                      selected={selected}
                    />
                    {selected && needsSubConfig(event.family) ? (
                      <EventSubConfig
                        family={event.family}
                        onChange={onSubConfigChange}
                        values={subConfigValues}
                      />
                    ) : null}
                  </Fragment>
                );
              })}
            </div>
          </div>
        ))
      )}
    </div>
  );
}
