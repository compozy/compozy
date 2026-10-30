/**
 * Computes the flow layout whenever its inputs change: the graph, the measured node sizes, the
 * measured label sizes and the canvas size. Each run is a request with an increasing id
 * (`layout-state.ts`); only the latest request for this flow writes positions, and a failure
 * waits for "Try again", which reruns with the same inputs. Nothing is persisted.
 */
import { useEffect, useEffectEvent, useReducer } from "react";

import type { FlowEntry } from "#gallery/flow";
import { computeLayout, type LayoutInput } from "#gallery/shell/flows/elk";
import { initialLayout, layoutReducer } from "#gallery/shell/flows/layout-state";

type Size = { width: number; height: number };

export const edgeId = (index: number) => `e${index}`;

type LayoutInputs = {
  flow: FlowEntry;
  nodeSizes: Record<string, Size>;
  labelSizes: Record<string, Size> | null;
  container: Size;
};

/** The ELK input, or `null` while some node, label or the canvas is not measured yet. */
export function layoutInput({
  flow,
  nodeSizes,
  labelSizes,
  container,
}: LayoutInputs): LayoutInput | null {
  if (!labelSizes || container.width <= 0 || container.height <= 0) return null;
  const nodes = flow.nodes.map(node => ({ id: node.id, ...nodeSizes[node.id] }));
  if (nodes.some(node => !node.width || !node.height)) return null;
  const edges = flow.edges.map((edge, index) => {
    const label = labelSizes[edgeId(index)];
    return label
      ? { id: edgeId(index), from: edge.from, to: edge.to, label: { text: edge.on, ...label } }
      : null;
  });
  if (edges.some(edge => edge === null)) return null;
  return {
    nodes: nodes as LayoutInput["nodes"],
    edges: edges as LayoutInput["edges"],
    // Rounded so sub-pixel resizes do not relayout.
    aspect: Math.round((container.width / container.height) * 10) / 10,
  };
}

export function useFlowLayout(inputs: LayoutInputs) {
  const flowId = inputs.flow.id;
  const [stored, dispatch] = useReducer(layoutReducer, flowId, initialLayout);
  // Another flow's state is never shown, even for the render before the reset lands.
  const state = stored.flow === flowId ? stored : initialLayout(flowId);
  const input = layoutInput(inputs);
  const key = input ? JSON.stringify(input) : null;

  useEffect(() => {
    dispatch({ type: "reset", flow: flowId });
  }, [flowId]);

  useEffect(() => {
    if (key) dispatch({ type: "request", flow: flowId });
  }, [key, flowId]);

  const run = useEffectEvent((request: number) => {
    if (!input || stored.flow !== flowId || stored.status !== "laying-out") return;
    computeLayout(input).then(
      layout => dispatch({ type: "resolve", flow: flowId, request, layout }),
      () => dispatch({ type: "reject", flow: flowId, request })
    );
  });

  useEffect(() => {
    if (stored.request > 0) run(stored.request);
  }, [stored.request]);

  return {
    status: state.status,
    layout: state.layout,
    retry: () => dispatch({ type: "retry" }),
  };
}
