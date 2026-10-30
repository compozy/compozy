/**
 * ELK layered layout of a flow, left to right in journey order: ELK places the nodes, routes the
 * edges orthogonally and positions the labels together, so parallel edges and loops stay readable.
 * One official worker runs the requests off the UI thread and is reused across them; a worker
 * error fails every pending request and replaces the worker for the next one. Its script is a separate asset,
 * keeping the layout engine out of the gallery's entry chunk.
 */
import ELK from "elkjs/lib/elk-api.js";
import workerUrl from "elkjs/lib/elk-worker.min.js?url";
import type { ElkExtendedEdge, ElkNode } from "elkjs/lib/elk-api";

import type { Layout, Point } from "#gallery/shell/flows/layout-state";

export type LayoutInput = {
  /** In journey (breadth-first) order; ELK keeps it as the model order. */
  nodes: { id: string; width: number; height: number }[];
  edges: {
    id: string;
    from: string;
    to: string;
    /** ELK only places labels that carry text. */
    label: { text: string; width: number; height: number };
  }[];
  /** Width / height of the canvas: long journeys wrap to its shape. */
  aspect: number;
};

const layoutOptions: Record<string, string> = {
  "elk.algorithm": "layered",
  "elk.direction": "RIGHT",
  "elk.edgeRouting": "ORTHOGONAL",
  "elk.layered.considerModelOrder.strategy": "NODES_AND_EDGES",
  "elk.layered.cycleBreaking.strategy": "MODEL_ORDER",
  "elk.layered.spacing.nodeNodeBetweenLayers": "32",
  "elk.layered.spacing.edgeNodeBetweenLayers": "20",
  "elk.layered.spacing.edgeEdgeBetweenLayers": "14",
  "elk.spacing.nodeNode": "40",
  "elk.spacing.edgeNode": "20",
  "elk.spacing.edgeEdge": "14",
  "elk.spacing.edgeLabel": "4",
  "elk.layered.edgeLabels.sideSelection": "ALWAYS_UP",
  // A long journey wraps into rows that follow the canvas shape instead of one thin line.
  "elk.layered.wrapping.strategy": "MULTI_EDGE",
  "elk.layered.wrapping.additionalEdgeSpacing": "24",
  "elk.padding": "[top=24,left=24,bottom=24,right=24]",
};

function points(edge: ElkExtendedEdge): Point[] {
  return (edge.sections ?? []).flatMap(section => [
    section.startPoint,
    ...(section.bendPoints ?? []),
    section.endPoint,
  ]);
}

type LayoutEngine = {
  elk: InstanceType<typeof ELK>;
  worker: Worker;
  failed: Promise<never>;
  /** Requests still waiting on this worker (timed-out ones no longer count). */
  pending: number;
  /** Detached after a timeout: no new requests; terminated once `pending` drains. */
  detached: boolean;
};

let engine: LayoutEngine | null = null;

function layoutEngine() {
  if (engine) return engine;
  const worker = new Worker(workerUrl);
  const failed = new Promise<never>((_, reject) => {
    worker.addEventListener("error", event =>
      reject(new Error(event.message || "Layout worker failed"))
    );
    worker.addEventListener("messageerror", () =>
      reject(new Error("Invalid layout worker response"))
    );
  });
  // Pending requests race against this; the next request starts a fresh worker.
  void failed.catch(() => {
    if (engine?.worker === worker) engine = null;
    worker.terminate();
  });
  engine = {
    elk: new ELK({ workerFactory: () => worker, algorithms: ["layered"] }),
    worker,
    failed,
    pending: 0,
    detached: false,
  };
  return engine;
}

/** A worker that never answers fails the request, so the flow can offer "Try again". */
const layoutTimeoutMs = 15_000;

function settle(current: LayoutEngine) {
  current.pending -= 1;
  if (current.detached && current.pending === 0) current.worker.terminate();
}

function layoutGraph(graph: ElkNode) {
  const current = layoutEngine();
  current.pending += 1;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let done = false;
  const finish = () => {
    if (done) return;
    done = true;
    clearTimeout(timer);
    settle(current);
  };
  const stalled = new Promise<never>((_, reject) => {
    timer = setTimeout(() => {
      // Later requests go to a fresh worker; this one lives until its other requests settle.
      if (engine === current) engine = null;
      current.detached = true;
      finish();
      reject(new Error("Layout timed out"));
    }, layoutTimeoutMs);
  });
  return Promise.race([current.elk.layout(graph), current.failed, stalled]).finally(finish);
}

export async function computeLayout(input: LayoutInput): Promise<Layout> {
  const graph: ElkNode = {
    id: "flow",
    layoutOptions: { ...layoutOptions, "elk.aspectRatio": input.aspect.toFixed(2) },
    children: input.nodes.map(node => ({ id: node.id, width: node.width, height: node.height })),
    edges: input.edges.map(edge => ({
      id: edge.id,
      sources: [edge.from],
      targets: [edge.to],
      labels: [
        {
          id: `${edge.id}:label`,
          text: edge.label.text,
          width: edge.label.width,
          height: edge.label.height,
          layoutOptions: { "elk.edgeLabels.placement": "CENTER" },
        },
      ],
    })),
  };
  const result = await layoutGraph(graph);
  const nodes: Layout["nodes"] = {};
  for (const child of result.children ?? []) {
    if (child.x === undefined || child.y === undefined) throw new Error(`No position: ${child.id}`);
    nodes[child.id] = { x: child.x, y: child.y };
  }
  const edges: Layout["edges"] = {};
  for (const edge of result.edges ?? []) {
    const route = points(edge);
    if (route.length < 2) throw new Error(`No route: ${edge.id}`);
    const label = edge.labels?.[0];
    edges[edge.id] = {
      points: route,
      label:
        label && label.x !== undefined && label.y !== undefined
          ? { x: label.x, y: label.y, width: label.width ?? 0, height: label.height ?? 0 }
          : null,
    };
  }
  for (const node of input.nodes) {
    if (!nodes[node.id]) throw new Error(`Node left out of the layout: ${node.id}`);
  }
  return { nodes, edges, width: result.width ?? 0, height: result.height ?? 0 };
}
