/**
 * Pure mapping from a resolved flow (plus its layout and selection) to React Flow nodes and
 * edges: the steps linked to the selected one stay lit, the rest dim.
 */
import type { FlowEntry, ResolvedNode } from "#gallery/flow";
import type { ReviewComment } from "#gallery/shell/comments/model";
import type { FlowEdgeType } from "#gallery/shell/flows/flow-edge";
import type { Layout } from "#gallery/shell/flows/layout-state";
import { nodeName } from "#gallery/shell/flows/node-model";
import type { ExternalNodeType, PrototypeNodeType } from "#gallery/shell/flows/nodes";
import { edgeId } from "#gallery/shell/flows/use-layout";
import { updateView, type Theme } from "#gallery/shell/view";

type Size = { width: number; height: number };

export const minZoom = 0.05;

export const selectNode = (id: string | null) => updateView({ node: id });

/** The steps linked to the selected one (itself included); the others are dimmed. */
export function relatedTo(flow: FlowEntry, selected: ResolvedNode | null) {
  const related = new Set<string>();
  if (!selected) return related;
  related.add(selected.id);
  for (const edge of flow.edges) {
    if (edge.from === selected.id) related.add(edge.to);
    if (edge.to === selected.id) related.add(edge.from);
  }
  return related;
}

function ariaLabel(flow: FlowEntry, item: ResolvedNode) {
  return [
    `Step ${item.step} of ${flow.nodes.length}: ${nodeName(item)}`,
    item.kind === "prototype" ? item.stateLabel : null,
    item.start ? "start of the flow" : null,
    item.end ? "end of the flow" : null,
  ]
    .filter(Boolean)
    .join(", ");
}

export function flowNodes(input: {
  flow: FlowEntry;
  layout: Layout | null;
  nodeSizes: Record<string, Size>;
  selected: ResolvedNode | null;
  theme: Theme | null;
  comments: ReviewComment[];
}): (PrototypeNodeType | ExternalNodeType)[] {
  const { flow, layout, selected } = input;
  const related = relatedTo(flow, selected);
  return flow.nodes.map(item => ({
    id: item.id,
    type: item.kind,
    position: layout?.nodes[item.id] ?? { x: 0, y: 0 },
    measured: input.nodeSizes[item.id],
    // Rendered hidden until laid out, so React Flow can measure them first.
    style: layout ? undefined : { visibility: "hidden" },
    className: "rounded-xl outline-none focus-visible:shadow-focus-ring",
    ariaLabel: ariaLabel(flow, item),
    data: {
      node: item,
      steps: flow.nodes.length,
      selected: item.id === selected?.id,
      dimmed: Boolean(selected) && !related.has(item.id),
      theme: input.theme,
      comments: input.comments.filter(
        comment => comment.status === "open" && comment.context.state === item.id
      ).length,
    },
  }));
}

export function flowEdges(
  flow: FlowEntry,
  layout: Layout | null,
  selected: ResolvedNode | null
): FlowEdgeType[] {
  return flow.edges.map((edge, index) => {
    const highlighted =
      Boolean(selected) && (edge.from === selected?.id || edge.to === selected?.id);
    return {
      id: edgeId(index),
      source: edge.from,
      target: edge.to,
      type: "flow",
      hidden: !layout,
      focusable: false,
      selectable: false,
      zIndex: highlighted ? 1 : 0,
      data: {
        edge,
        route: layout?.edges[edgeId(index)] ?? null,
        highlighted,
        dimmed: Boolean(selected) && !highlighted,
      },
    };
  });
}
