/**
 * Flow canvas (`?flow=`): a read-only React Flow view of one flow with the ELK layout, legend,
 * warnings, zoom and the side panel (selected step and the flow's comments, resizable). Steps
 * show their open comment counts. Selection lives in the URL (`?node=`): click or Enter selects,
 * double-click or Shift+Enter opens the prototype, Escape or a click on the empty canvas clears it.
 */
import "@xyflow/react/dist/style.css";

import { Background, BackgroundVariant, ReactFlow, ReactFlowProvider } from "@xyflow/react";
import { useEffect, useRef } from "react";

import type { FlowEntry } from "#gallery/flow";
import type { ReviewComment } from "#gallery/shell/comments/model";
import {
  FlowLegend,
  FlowWarnings,
  LayoutStatusRegion,
  ZoomControls,
} from "#gallery/shell/flows/chrome";
import { selectNode } from "#gallery/shell/flows/canvas-model";
import { edgeLabelClass } from "#gallery/shell/flows/edge-style";
import { FlowEdgeView, FlowMarkers } from "#gallery/shell/flows/flow-edge";
import { openNode } from "#gallery/shell/flows/node-model";
import { ExternalNodeView, PrototypeNodeView } from "#gallery/shell/flows/nodes";
import { minZoom } from "#gallery/shell/flows/canvas-model";
import { PreviewQueue } from "#gallery/shell/flows/previews";
import { FlowSide } from "#gallery/shell/flows/side";
import { useFlowCanvas } from "#gallery/shell/flows/use-flow-canvas";
import { edgeId } from "#gallery/shell/flows/use-layout";
import { flowWarnings, type Registry } from "#gallery/shell/registry-model";
import { WithSidePanel } from "#gallery/shell/side-panel";
import type { Theme } from "#gallery/shell/view";

type Size = { width: number; height: number };

export type FlowViewProps = {
  flow: FlowEntry;
  node: string | null;
  registry: Registry;
  theme: Theme | null;
  dark: boolean;
  comments: ReviewComment[];
  commentsError: string | null;
  /** Comments tab of the side panel (`?comments=1`) and the focused comment. */
  panel: boolean;
  comment: string | null;
  /** Side panel shown. */
  side: boolean;
  onSide: (open: boolean) => void;
};

const nodeTypes = { prototype: PrototypeNodeView, external: ExternalNodeView };
const edgeTypes = { flow: FlowEdgeView };

/** Measures every edge label with the canvas markup, so ELK reserves their real size. */
function LabelMeasurer({
  flow,
  onSizes,
}: {
  flow: FlowEntry;
  onSizes: (sizes: Record<string, Size>) => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const measure = () => {
      const sizes: Record<string, Size> = {};
      for (const element of ref.current?.querySelectorAll<HTMLElement>("[data-edge]") ?? []) {
        sizes[element.dataset.edge ?? ""] = {
          width: Math.ceil(element.offsetWidth),
          height: Math.ceil(element.offsetHeight),
        };
      }
      onSizes(sizes);
    };
    measure();
    // Labels widen once the product fonts load; measure again then.
    void document.fonts.ready.then(measure);
  }, [flow, onSizes]);
  return (
    <div
      aria-hidden="true"
      className="invisible absolute top-0 left-0 -z-10 flex flex-col items-start"
      ref={ref}
    >
      {flow.edges.map((edge, index) => (
        <span className={edgeLabelClass} data-edge={edgeId(index)} key={edgeId(index)}>
          {edge.on}
        </span>
      ))}
    </div>
  );
}

function FlowCanvas(props: FlowViewProps) {
  const { flow, registry, theme, comments } = props;
  const canvas = useFlowCanvas(props);
  const { status, layout, retry, selected, nodes, edges, containerRef } = canvas;

  const side = (
    <FlowSide
      comments={comments}
      dark={props.dark}
      error={props.commentsError}
      flow={flow}
      node={selected}
      onClose={() => props.onSide(false)}
      onSelectNode={canvas.goToNode}
      panel={props.panel}
      registry={registry}
      selectedComment={props.comment}
      theme={theme}
    />
  );

  return (
    <WithSidePanel id="flow" label="Flow panel width" side={props.side ? side : null}>
      <div className="flex min-h-0 min-w-0 flex-1" data-flow={flow.id} data-testid="flow-view">
        <div
          className="relative min-h-0 min-w-0 flex-1 overflow-hidden rounded-xl bg-surface-2"
          data-layout-status={status}
          data-testid="flow-canvas"
          onKeyDown={canvas.onKeyDown}
          ref={containerRef}
          role="presentation"
        >
          <FlowMarkers />
          <LabelMeasurer flow={flow} onSizes={canvas.setLabelSizes} />
          <PreviewQueue ready={Boolean(layout)}>
            <ReactFlow
              aria-label={`Flow ${flow.title}`}
              attributionPosition="bottom-left"
              className="[--xy-background-color:transparent]"
              deleteKeyCode={null}
              edges={edges}
              edgesFocusable={false}
              edgeTypes={edgeTypes}
              elementsSelectable={false}
              maxZoom={1.6}
              minZoom={minZoom}
              multiSelectionKeyCode={null}
              nodes={nodes}
              nodesConnectable={false}
              nodesDraggable={false}
              nodeTypes={nodeTypes}
              onNodeClick={(_, item) => selectNode(item.id)}
              onNodeDoubleClick={(_, item) => {
                const target = flow.nodes.find(entry => entry.id === item.id);
                if (target) openNode(target);
              }}
              onNodesChange={canvas.onNodesChange}
              onPaneClick={() => selectNode(null)}
              proOptions={{ hideAttribution: true }}
              selectionKeyCode={null}
              zoomOnDoubleClick={false}
            >
              <Background
                color="var(--color-line-strong)"
                gap={18}
                size={1.2}
                variant={BackgroundVariant.Dots}
              />
            </ReactFlow>
          </PreviewQueue>
          <div className="pointer-events-none absolute inset-x-3 top-3 flex items-start justify-between gap-3">
            <FlowLegend />
            <FlowWarnings warnings={flowWarnings(registry, flow)} />
          </div>
          <div className="pointer-events-none absolute right-3 bottom-3">
            <ZoomControls onFit={() => canvas.focusOn(null)} />
          </div>
          {layout && status !== "failed" ? null : (
            <LayoutStatusRegion onRetry={retry} status={status} />
          )}
        </div>
      </div>
    </WithSidePanel>
  );
}

export function FlowView(props: FlowViewProps) {
  return (
    // A new flow starts from scratch: no positions, sizes or viewport of the previous one.
    <ReactFlowProvider key={props.flow.id}>
      <FlowCanvas {...props} />
    </ReactFlowProvider>
  );
}
