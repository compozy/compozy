/**
 * Prototype toolbar "In N flows" (N = distinct flows that show the state on screen): a menu with
 * every occurrence (flow and "step X of Y"), each opening the flow with that node selected. "Not
 * in a flow" when the state is in none, with "Other states in flows" when other states of the
 * prototype are; no menu at all when nothing of the prototype is in a flow.
 */
import { ChevronDownIcon, WorkflowIcon } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@compozy/ui";

import type { Occurrence } from "#gallery/flow";
import { flowCount } from "#gallery/flow-graph";
import { flowById, occurrencesOf, type Entry, type Registry } from "#gallery/shell/registry-model";
import { SelectionLink } from "#gallery/shell/selection-link";

function OccurrenceItem({
  registry,
  occurrence,
  detail,
}: {
  registry: Registry;
  occurrence: Occurrence;
  detail?: string;
}) {
  const title = flowById(registry, occurrence.flow)?.title ?? occurrence.flow;
  const position = `step ${occurrence.step} of ${occurrence.steps}`;
  return (
    <DropdownMenuItem
      className="flex-col items-start gap-0"
      render={
        <SelectionLink
          aria-label={[title, detail, position].filter(Boolean).join(", ")}
          to={{ kind: "flow", flow: occurrence.flow, node: occurrence.node }}
        />
      }
    >
      <span>{title}</span>
      {detail ? <span className="text-small-body text-muted">{detail}</span> : null}
      <span className="font-mono text-mono-id tracking-mono-id text-subtle">{position}</span>
    </DropdownMenuItem>
  );
}

export function FlowMembership({
  registry,
  entry,
  state,
}: {
  registry: Registry;
  entry: Entry;
  state: string;
}) {
  const here = occurrencesOf(registry, entry.slug, state);
  const others = entry.meta.states.flatMap(option =>
    option.id === state
      ? []
      : occurrencesOf(registry, entry.slug, option.id).map(occurrence => ({ option, occurrence }))
  );
  const count = flowCount(here);
  const label = count === 0 ? "Not in a flow" : `In ${count} ${count === 1 ? "flow" : "flows"}`;

  if (here.length === 0 && others.length === 0) {
    return (
      <span
        className="flex h-7 items-center gap-1.5 px-2 text-small-body text-muted"
        data-testid="flow-membership"
      >
        <WorkflowIcon aria-hidden="true" className="size-3.5" />
        {label}
      </span>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<Button data-testid="flow-membership" size="sm" variant="quiet" />}
      >
        <WorkflowIcon aria-hidden="true" />
        {label}
        <ChevronDownIcon aria-hidden="true" className="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-80">
        {here.length > 0 ? (
          <DropdownMenuGroup>
            <DropdownMenuLabel>This state appears in</DropdownMenuLabel>
            {here.map(occurrence => (
              <OccurrenceItem
                key={`${occurrence.flow}:${occurrence.node}`}
                occurrence={occurrence}
                registry={registry}
              />
            ))}
          </DropdownMenuGroup>
        ) : null}
        {here.length > 0 && others.length > 0 ? <DropdownMenuSeparator /> : null}
        {others.length > 0 ? (
          <DropdownMenuGroup>
            <DropdownMenuLabel>Other states in flows</DropdownMenuLabel>
            {others.map(({ option, occurrence }) => (
              <OccurrenceItem
                detail={option.label}
                key={`${option.id}:${occurrence.flow}:${occurrence.node}`}
                occurrence={occurrence}
                registry={registry}
              />
            ))}
          </DropdownMenuGroup>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
