/**
 * Flow rows of the sidebar: the global "Journeys" group (level 1), the spec level's "Flows" and
 * "Crossing this spec" groups (level 2) and flow search results. Every row is a numbered link
 * with its own URL, like the prototype rows.
 */
import { TriangleAlertIcon } from "lucide-react";

import { flowKey, type FlowEntry } from "#gallery/flow";
import { flowWarnings, plural, type Group, type Registry } from "#gallery/shell/registry-model";
import { CommentCount, NavRow, RowGroup, type OpenOf } from "#gallery/shell/sidebar-levels";

function Warnings({ count }: { count: number }) {
  if (count === 0) return null;
  return (
    <span
      className="flex items-center gap-0.5 text-warning"
      title={`${plural(count, "state")} outside every flow`}
    >
      <TriangleAlertIcon aria-hidden="true" className="size-3" />
      {count}
      <span className="sr-only">coverage warnings</span>
    </span>
  );
}

function FlowRow({
  flow,
  index,
  detail,
  active,
  warnings = 0,
  openComments = 0,
  onNavigate,
}: {
  flow: FlowEntry;
  /** Position in its list; search results have none. */
  index: number | null;
  detail: string;
  active: boolean;
  warnings?: number;
  openComments?: number;
  onNavigate?: () => void;
}) {
  return (
    <NavRow
      active={active}
      aside={
        <>
          <CommentCount count={openComments} />
          <Warnings count={warnings} />
        </>
      }
      detail={detail}
      index={index}
      onNavigate={onNavigate}
      target={{ kind: "flow", flow: flow.id }}
      testId="nav-flow"
      title={flow.title}
    />
  );
}

/** Level 1: every flow, by owner spec then title. */
export function JourneyRows({
  registry,
  active,
  openOf,
}: {
  registry: Registry;
  active: string | null;
  openOf: OpenOf;
}) {
  if (registry.flows.length === 0) return null;
  return (
    <RowGroup aside={registry.flows.length} label="Journeys">
      {registry.flows.map((flow, index) => (
        <FlowRow
          active={flow.id === active}
          detail={`${flow.spec ?? "no spec"} · ${plural(flow.nodes.length, "step")}`}
          flow={flow}
          index={index}
          key={flow.id}
          openComments={openOf(flowKey(flow.id))}
        />
      ))}
    </RowGroup>
  );
}

/** Level 2: flows the spec owns, then flows owned elsewhere that cross it. */
export function SpecFlowRows({
  group,
  registry,
  active,
  openOf,
}: {
  group: Group;
  registry: Registry;
  active: string | null;
  openOf: OpenOf;
}) {
  return (
    <>
      {group.flows.length > 0 ? (
        <RowGroup aside={group.flows.length} label="Flows">
          {group.flows.map((flow, index) => (
            <FlowRow
              active={flow.id === active}
              detail={`${plural(flow.nodes.length, "step")} · ${
                flow.specs.length > 1 ? `crosses ${plural(flow.specs.length, "spec")}` : flow.id
              }`}
              flow={flow}
              index={index}
              key={flow.id}
              openComments={openOf(flowKey(flow.id))}
              warnings={flowWarnings(registry, flow).length}
            />
          ))}
        </RowGroup>
      ) : null}
      {group.crossing.length > 0 ? (
        <RowGroup aside={group.crossing.length} label="Crossing this spec">
          {group.crossing.map((flow, index) => (
            <FlowRow
              active={flow.id === active}
              detail={`${flow.spec ?? "no spec"} · ${plural(flow.nodes.length, "step")}`}
              flow={flow}
              index={index}
              key={flow.id}
              openComments={openOf(flowKey(flow.id))}
            />
          ))}
        </RowGroup>
      ) : null}
    </>
  );
}

/** Flow matches of the sidebar search. */
export function FlowResults({
  flows,
  onNavigate,
}: {
  flows: readonly FlowEntry[];
  onNavigate: () => void;
}) {
  if (flows.length === 0) return null;
  return (
    <RowGroup aside={flows.length} label="Flows">
      {flows.map(flow => (
        <FlowRow
          active={false}
          detail={`${flow.spec ?? "no spec"} · ${flow.id}`}
          flow={flow}
          index={null}
          key={flow.id}
          onNavigate={onNavigate}
        />
      ))}
    </RowGroup>
  );
}
