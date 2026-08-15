import { useEffect, type CSSProperties } from "react";
import {
  Background,
  BackgroundVariant,
  ReactFlow,
  useReactFlow,
  type Connection,
  type EdgeChange,
  type IsValidConnection,
  type NodeChange,
  type NodeTypes,
} from "@xyflow/react";

import "@xyflow/react/dist/style.css";

import type { EditorEdge, EditorNode } from "../../lib/codec";
import { EDITOR_NODE_HEIGHT, EDITOR_NODE_WIDTH } from "../../lib/loop-editor-layout";
import type { LoopEnvironmentSpec } from "../../types";
import { LoopEditorNode } from "./loop-editor-node";

const nodeTypes: NodeTypes = { loopNode: LoopEditorNode };
const isValidConnection: IsValidConnection<EditorEdge> = connection =>
  connection.source !== connection.target;

const FLOW_TOKEN_STYLE = {
  "--xy-background-color-default": "var(--color-canvas)",
  "--xy-edge-stroke-default": "color-mix(in srgb, var(--color-fg) 20%, var(--color-canvas))",
  "--xy-edge-stroke-width-default": "1.5",
  "--xy-handle-background-color-default":
    "color-mix(in srgb, var(--color-fg) 24%, var(--color-canvas))",
  "--xy-handle-border-color-default": "var(--color-line)",
} as CSSProperties;

interface LoopEditorCanvasProps {
  nodes: EditorNode[];
  edges: EditorEdge[];
  /** The view-model's selected node — the single source of truth for the accent ring, so
   *  reveal-from-dock and palette-add show the ring, not just a direct canvas click. */
  selectedNodeId: string | null;
  /** Bumped only by Reveal node so the canvas can pan without storing a flow handle. */
  revealSeq: number;
  onNodesChange: (changes: NodeChange<EditorNode>[]) => void;
  onEdgesChange: (changes: EdgeChange<EditorEdge>[]) => void;
  onConnect: (connection: Connection) => void;
  onSelectNode: (id: string | null) => void;
  /**
   * A non-workspace definition is immutable: no new edges, no reconnects, no delete key.
   * Selection, panning, zoom and dragging stay on — reading the graph is not a mutation, and
   * positions live in the annotations sidecar the daemon accepts for any source.
   */
  readOnly: boolean;
  loopDefaultEnvironment?: LoopEnvironmentSpec;
}

/**
 * The `@xyflow/react` DAG canvas (design §4.6). Controlled flow: nodes/edges +
 * change handlers live in the editor view-model, and this component only renders
 * + reports interaction. `isValidConnection` is a connect-time UX hint (no
 * self-loops) — never an invariant check; the shared Go linter owns acyclicity
 * (ADR-015).
 */
export function LoopEditorCanvas({
  nodes,
  edges,
  selectedNodeId,
  revealSeq,
  onNodesChange,
  onEdgesChange,
  onConnect,
  onSelectNode,
  readOnly,
  loopDefaultEnvironment,
}: LoopEditorCanvasProps) {
  const { getZoom, setCenter } = useReactFlow();
  // Drive the accent ring from the view-model selection so every selection path (click,
  // dock reveal, palette add) is truthful — React Flow's internal `selected` only tracks clicks.
  const displayNodes = nodes.map(node => ({
    ...node,
    selected: node.id === selectedNodeId,
    data: { ...node.data, loopDefaultEnvironment, readOnly },
  }));

  useEffect(() => {
    if (revealSeq === 0 || !selectedNodeId) return;
    const node = nodes.find(entry => entry.id === selectedNodeId);
    if (!node) return;
    void setCenter(
      node.position.x + EDITOR_NODE_WIDTH / 2,
      node.position.y + EDITOR_NODE_HEIGHT / 2,
      { duration: 200, zoom: getZoom() }
    );
  }, [getZoom, nodes, revealSeq, selectedNodeId, setCenter]);

  return (
    <ReactFlow
      // Dark-only for v1 (CompozyOS ships a single dark theme); wire `colorMode` to the app theme
      // if a light theme lands.
      colorMode="dark"
      nodes={displayNodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onConnect={onConnect}
      isValidConnection={isValidConnection}
      nodesConnectable={!readOnly}
      edgesReconnectable={!readOnly}
      deleteKeyCode={readOnly ? null : undefined}
      onNodeClick={(_event, node) => onSelectNode(node.id)}
      onPaneClick={() => onSelectNode(null)}
      fitView
      minZoom={0.4}
      maxZoom={1.6}
      proOptions={{ hideAttribution: true }}
      data-testid="loop-editor-canvas"
      style={FLOW_TOKEN_STYLE}
    >
      <Background variant={BackgroundVariant.Dots} gap={22} size={1} className="opacity-60" />
    </ReactFlow>
  );
}
