/**
 * State and handlers of the flow canvas: measured node and label sizes feed the ELK layout, the
 * viewport frames each new layout, and keyboard/selection handlers keep `?node=` current.
 */
import { getViewportForBounds, useReactFlow, type NodeChange } from "@xyflow/react";
import { useEffect, useEffectEvent, useState, type KeyboardEvent } from "react";

import { flowEdges, flowNodes, minZoom, selectNode } from "#gallery/shell/flows/canvas-model";
import { viewportDuration } from "#gallery/shell/flows/edge-style";
import type { FlowViewProps } from "#gallery/shell/flows/flow-view";
import { openNode } from "#gallery/shell/flows/node-model";
import { useFlowLayout } from "#gallery/shell/flows/use-layout";
import { useElementSize } from "#gallery/shell/use-element-size";

type Size = { width: number; height: number };

export function useFlowCanvas(props: FlowViewProps) {
  const { flow, node: selectedId, theme, comments } = props;
  const { setViewport, setCenter, getZoom } = useReactFlow();
  const [containerRef, container] = useElementSize<HTMLDivElement>();
  const [nodeSizes, setNodeSizes] = useState<Record<string, Size>>({});
  const [labelSizes, setLabelSizes] = useState<Record<string, Size> | null>(null);
  const { status, layout, retry } = useFlowLayout({ flow, nodeSizes, labelSizes, container });
  const selected = flow.nodes.find(item => item.id === selectedId) ?? null;

  const nodes = flowNodes({ flow, layout, nodeSizes, selected, theme, comments });
  const edges = flowEdges(flow, layout, selected);

  const onNodesChange = (changes: NodeChange[]) => {
    const measured = changes.flatMap(change =>
      change.type === "dimensions" && change.dimensions
        ? [{ id: change.id, ...change.dimensions }]
        : []
    );
    if (measured.length === 0) return;
    setNodeSizes(current => {
      const next = { ...current };
      for (const { id, width, height } of measured) next[id] = { width, height };
      return next;
    });
  };

  const focusOn = (id: string | null) => {
    const position = id ? layout?.nodes[id] : null;
    const size = id ? nodeSizes[id] : null;
    if (position && size) {
      void setCenter(position.x + size.width / 2, position.y + size.height / 2, {
        zoom: Math.max(getZoom(), 0.8),
        duration: viewportDuration(),
      });
    } else if (layout) {
      // ELK's bounds include routed edges and labels that extend beyond the node cards.
      const viewport = getViewportForBounds(
        { x: 0, y: 0, width: layout.width, height: layout.height },
        container.width,
        container.height,
        minZoom,
        1,
        { top: "64px", bottom: "56px", left: "24px", right: "24px" }
      );
      void setViewport(viewport, { duration: viewportDuration() });
    }
  };

  // Only a new layout (first one, resize, remeasure) moves the viewport, to the selected node or
  // the whole flow; selecting on the canvas does not.
  const frameLayout = useEffectEvent(() => focusOn(selectedId));
  useEffect(() => {
    if (!layout) return;
    const frame = requestAnimationFrame(() => frameLayout());
    return () => cancelAnimationFrame(frame);
  }, [layout]);

  const goToNode = (id: string) => {
    selectNode(id);
    focusOn(id);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Escape" && selected) {
      event.preventDefault();
      selectNode(null);
      return;
    }
    if (event.key !== "Enter") return;
    const element = (event.target as HTMLElement).closest<HTMLElement>(".react-flow__node");
    const target = flow.nodes.find(item => item.id === element?.dataset.id);
    if (!target) return;
    event.preventDefault();
    if (event.shiftKey) openNode(target);
    else selectNode(target.id);
  };

  return {
    containerRef,
    status,
    layout,
    retry,
    selected,
    nodes,
    edges,
    setLabelSizes,
    onNodesChange,
    focusOn,
    goToNode,
    onKeyDown,
  };
}
