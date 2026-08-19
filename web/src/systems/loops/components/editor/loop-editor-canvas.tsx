import { useEffect, useRef, type CSSProperties, type MouseEvent as ReactMouseEvent } from "react";
import {
  Background,
  BackgroundVariant,
  ReactFlow,
  useReactFlow,
  type Connection,
  type EdgeChange,
  type EdgeTypes,
  type IsValidConnection,
  type NodeChange,
  type NodeTypes,
  type OnConnectEnd,
} from "@xyflow/react";

import "@xyflow/react/dist/style.css";

import type { EditorEdge, EditorNode } from "../../lib/codec";
import {
  EDITOR_NODE_HEIGHT,
  EDITOR_NODE_WIDTH,
  editorNodeHeight,
} from "../../lib/loop-editor-layout";
import { buildDisplayEdges, LOOP_EDITOR_EDGE_TYPE } from "../../lib/loop-editor-route-edges";
import type { LoopEditorConnectionDrop } from "../../lib/loop-editor-types";
import type { LoopEnvironmentSpec } from "../../types";
import { LoopEditorEdge } from "./loop-editor-edge";
import { LoopEditorNode } from "./loop-editor-node";

const nodeTypes: NodeTypes = { loopNode: LoopEditorNode };
const edgeTypes: EdgeTypes = { [LOOP_EDITOR_EDGE_TYPE]: LoopEditorEdge };
const isValidConnection: IsValidConnection<EditorEdge> = connection =>
  connection.source !== connection.target;

const PANE_CLASS = "react-flow__pane";

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

  onNodesDelete?: (deleted: EditorNode[]) => void;

  onConnectDropped?: (drop: LoopEditorConnectionDrop) => void;

  onQuickAdd?: (position: { x: number; y: number }) => void;
  /**
   * A non-workspace definition is immutable: no new edges, no reconnects, no delete key.
   * Selection, panning, zoom and dragging stay on — reading the graph is not a mutation, and
   * positions live in the annotations sidecar the daemon accepts for any source.
   */
  readOnly: boolean;
  loopDefaultEnvironment?: LoopEnvironmentSpec;
}

function releasePoint(event: MouseEvent | TouchEvent): { x: number; y: number } | null {
  if ("changedTouches" in event) {
    const touch = event.changedTouches[0];
    return touch ? { x: touch.clientX, y: touch.clientY } : null;
  }
  return { x: event.clientX, y: event.clientY };
}

export function LoopEditorCanvas({
  nodes,
  edges,
  selectedNodeId,
  revealSeq,
  onNodesChange,
  onEdgesChange,
  onConnect,
  onSelectNode,
  onNodesDelete,
  onConnectDropped,
  onQuickAdd,
  readOnly,
  loopDefaultEnvironment,
}: LoopEditorCanvasProps) {
  const { getZoom, screenToFlowPosition, setCenter } = useReactFlow();
  const lastHandledSeq = useRef(0);
  const displayNodes = nodes.map(node => ({
    ...node,
    data: { ...node.data, loopDefaultEnvironment, readOnly, focused: node.id === selectedNodeId },
  }));

  const displayEdges = buildDisplayEdges(edges, nodes, {
    readOnly,
    onDelete: edgeId => onEdgesChange([{ id: edgeId, type: "remove" }]),
  });

  const centeredPosition = (point: { x: number; y: number }) => {
    const flowPoint = screenToFlowPosition(point);
    return { x: flowPoint.x - EDITOR_NODE_WIDTH / 2, y: flowPoint.y - EDITOR_NODE_HEIGHT / 2 };
  };

  const handleConnectEnd: OnConnectEnd = (event, connectionState) => {
    if (readOnly || !onConnectDropped) return;

    if (connectionState.isValid || connectionState.toNode !== null) return;
    const source = connectionState.fromNode?.id;
    if (!source) return;
    const point = releasePoint(event);
    if (!point) return;
    const drop = { source, point, position: centeredPosition(point) };
    window.setTimeout(() => onConnectDropped(drop), 0);
  };

  const handleDoubleClick = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (!onQuickAdd) return;

    if (!(event.target instanceof Element) || !event.target.classList.contains(PANE_CLASS)) return;
    onQuickAdd(centeredPosition({ x: event.clientX, y: event.clientY }));
  };

  useEffect(() => {
    if (revealSeq === 0 || !selectedNodeId) return;
    if (revealSeq <= lastHandledSeq.current) return;
    const node = nodes.find(entry => entry.id === selectedNodeId);
    if (!node) return;
    lastHandledSeq.current = revealSeq;
    void setCenter(
      node.position.x + EDITOR_NODE_WIDTH / 2,

      node.position.y + editorNodeHeight(node) / 2,
      { duration: 200, zoom: getZoom() }
    );
  }, [getZoom, nodes, revealSeq, selectedNodeId, setCenter]);

  return (
    <ReactFlow
      // Dark-only for v1 (CompozyOS ships a single dark theme); wire `colorMode` to the app theme
      // if a light theme lands.
      colorMode="dark"
      nodes={displayNodes}
      edges={displayEdges}
      nodeTypes={nodeTypes}
      edgeTypes={edgeTypes}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onNodesDelete={onNodesDelete}
      onConnect={onConnect}
      onConnectEnd={handleConnectEnd}
      onDoubleClickCapture={handleDoubleClick}
      isValidConnection={isValidConnection}
      nodesConnectable={!readOnly}
      edgesReconnectable={!readOnly}
      deleteKeyCode={readOnly ? null : ["Backspace", "Delete"]}
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
