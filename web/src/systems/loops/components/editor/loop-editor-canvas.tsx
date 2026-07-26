import {
  Background,
  BackgroundVariant,
  ReactFlow,
  type Connection,
  type EdgeChange,
  type IsValidConnection,
  type NodeChange,
  type NodeTypes,
} from "@xyflow/react";

import "@xyflow/react/dist/style.css";

import type { EditorEdge, EditorNode } from "../../lib/codec";
import { LoopEditorNode } from "./loop-editor-node";

const nodeTypes: NodeTypes = { loopNode: LoopEditorNode };
const isValidConnection: IsValidConnection<EditorEdge> = connection =>
  connection.source !== connection.target;

interface LoopEditorCanvasProps {
  nodes: EditorNode[];
  edges: EditorEdge[];
  /** The view-model's selected node — the single source of truth for the accent ring, so
   *  reveal-from-dock and palette-add show the ring, not just a direct canvas click. */
  selectedNodeId: string | null;
  onNodesChange: (changes: NodeChange<EditorNode>[]) => void;
  onEdgesChange: (changes: EdgeChange<EditorEdge>[]) => void;
  onConnect: (connection: Connection) => void;
  onSelectNode: (id: string | null) => void;
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
  onNodesChange,
  onEdgesChange,
  onConnect,
  onSelectNode,
}: LoopEditorCanvasProps) {
  // Drive the accent ring from the view-model selection so every selection path (click,
  // dock reveal, palette add) is truthful — React Flow's internal `selected` only tracks clicks.
  const displayNodes = nodes.map(node =>
    node.selected === (node.id === selectedNodeId)
      ? node
      : { ...node, selected: node.id === selectedNodeId }
  );
  return (
    <ReactFlow
      // Dark-only for v1 (AGH ships a single dark theme); wire `colorMode` to the app theme
      // if a light theme lands.
      colorMode="dark"
      nodes={displayNodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onConnect={onConnect}
      isValidConnection={isValidConnection}
      onNodeClick={(_event, node) => onSelectNode(node.id)}
      onPaneClick={() => onSelectNode(null)}
      fitView
      minZoom={0.4}
      maxZoom={1.6}
      proOptions={{ hideAttribution: true }}
      data-testid="loop-editor-canvas"
    >
      <Background variant={BackgroundVariant.Dots} gap={22} size={1} className="opacity-60" />
    </ReactFlow>
  );
}
