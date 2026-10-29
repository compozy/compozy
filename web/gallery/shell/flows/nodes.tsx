/**
 * Flow node cards. A prototype node shows its title, owning spec, a fixed 16:10 preview area and
 * its route and state; the canvas preview queue (`previews.tsx`) decides when the live preview
 * mounts. An external node never embeds its page: dashed outline (not the web app), service
 * initial, label and URL. Node size never depends on zoom or preview state, so the layout stays
 * stable.
 */
import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import { MessageSquareIcon } from "lucide-react";

import { Pill, cn } from "@compozy/ui";

import { externalLabels, type ResolvedNode } from "#gallery/flow";
import { displayUrl, externalName } from "#gallery/shell/flows/node-model";
import { PreviewFrame } from "#gallery/shell/flows/preview-frame";
import { usePreview } from "#gallery/shell/flows/use-preview";
import { frameSrc, type Theme } from "#gallery/shell/view";

export type FlowNodeData = {
  node: ResolvedNode;
  steps: number;
  selected: boolean;
  dimmed: boolean;
  theme: Theme | null;
  /** Open review comments on this step. */
  comments: number;
};
export type PrototypeNodeType = Node<FlowNodeData, "prototype">;
export type ExternalNodeType = Node<FlowNodeData, "external">;

/** Card width at 100 %; the preview area keeps 16:10 of a 1440 px page. */
const cardWidth = 240;
const previewWidth = cardWidth - 16;
const previewHeight = Math.round((previewWidth * 10) / 16);

const cardState = (data: FlowNodeData) =>
  cn(
    "relative transition-opacity duration-base motion-reduce:transition-none",
    data.dimmed && "opacity-40",
    data.selected && "outline-2 outline-offset-2 outline-accent"
  );

function NodeComments({ count }: { count: number }) {
  if (count === 0) return null;
  return (
    <span
      className="flex shrink-0 items-center gap-0.5 font-mono text-mono-id tracking-mono-id text-accent"
      data-testid="flow-node-comments"
      title={`${count} open ${count === 1 ? "comment" : "comments"}`}
    >
      <MessageSquareIcon aria-hidden="true" className="size-3" />
      {count}
    </span>
  );
}

/** Handles exist only because React Flow anchors edges to them; ELK owns the routes. */
function Anchors() {
  return (
    <>
      <Handle className="invisible" isConnectable={false} position={Position.Left} type="target" />
      <Handle className="invisible" isConnectable={false} position={Position.Right} type="source" />
    </>
  );
}

function Tags({ node }: { node: ResolvedNode }) {
  return (
    <>
      {node.start ? (
        <Pill className="absolute -top-2.5 -left-2" mono solid tone="neutral">
          start
        </Pill>
      ) : null}
      {node.end ? (
        <Pill className="absolute -top-2.5 -right-2" mono solid tone="accent">
          end
        </Pill>
      ) : null}
    </>
  );
}

function PrototypeCard({
  node,
  src,
  data,
}: {
  node: Extract<ResolvedNode, { kind: "prototype" }>;
  src: string;
  data: FlowNodeData;
}) {
  const preview = usePreview(node.id, src);
  return (
    <div
      className={cn(cardState(data), "rounded-xl border border-line bg-card p-2 text-fg")}
      data-node={node.id}
      data-testid="flow-node"
      style={{ width: cardWidth }}
    >
      <Anchors />
      <Tags node={node} />
      <div className="flex items-baseline justify-between gap-2 px-0.5 pb-1.5">
        <span className="min-w-0 flex-1 truncate text-small-body font-medium">{node.title}</span>
        <NodeComments count={data.comments} />
        {node.spec ? (
          <span className="shrink-0 font-mono text-mono-id tracking-mono-id text-subtle">
            {node.spec}
          </span>
        ) : null}
      </div>
      {/* Fixed size: the preview's layout and paint never leave this box. */}
      <div
        className="relative overflow-hidden rounded-md bg-canvas [contain:strict]"
        style={{ width: previewWidth, height: previewHeight }}
      >
        {preview.mounted ? (
          <PreviewFrame
            height={previewHeight}
            key={src}
            onSettle={preview.onSettle}
            src={src}
            title={`${node.title} · ${node.stateLabel}`}
            width={previewWidth}
          />
        ) : (
          <div className="flex h-full flex-col justify-center gap-1 px-3 text-center">
            <span className="text-small-body">{node.stateLabel}</span>
            <span className="font-mono text-mono-id tracking-mono-id text-subtle">
              step {node.step} of {data.steps}
            </span>
          </div>
        )}
      </div>
      <div className="flex justify-between gap-2 px-0.5 pt-1.5 font-mono text-mono-id tracking-mono-id">
        <span className="truncate text-subtle">{node.route ?? `/${node.proto}`}</span>
        <span className="shrink-0 text-accent">{node.state}</span>
      </div>
    </div>
  );
}

export function PrototypeNodeView({ data }: NodeProps<PrototypeNodeType>) {
  const node = data.node;
  if (node.kind !== "prototype") return null;
  const src = frameSrc(node.proto, { state: node.state, theme: data.theme });
  return <PrototypeCard data={data} node={node} src={src} />;
}

export function ExternalNodeView({ data }: NodeProps<ExternalNodeType>) {
  const node = data.node;
  if (node.kind !== "external") return null;
  const name = externalName(node);
  const service = externalLabels[node.external];
  return (
    <div
      className={cn(
        cardState(data),
        "flex w-52 flex-col gap-1 rounded-xl border border-dashed border-line-strong bg-surface-2 p-3"
      )}
      data-node={node.id}
      data-testid="flow-node"
    >
      <Anchors />
      <Tags node={node} />
      <span
        aria-hidden="true"
        className="mb-1 grid size-7 place-items-center rounded-md bg-hover font-mono text-small-body font-medium"
      >
        {(service || name).charAt(0).toUpperCase()}
      </span>
      <span className="flex items-baseline justify-between gap-2 text-small-body">
        {name}
        <NodeComments count={data.comments} />
      </span>
      <span className="text-small-body text-muted">
        {service && service !== name ? `${service} · outside the web app` : "outside the web app"}
      </span>
      {node.url ? (
        <span className="truncate font-mono text-mono-id tracking-mono-id text-subtle">
          {displayUrl(node.url)}
        </span>
      ) : null}
    </div>
  );
}
