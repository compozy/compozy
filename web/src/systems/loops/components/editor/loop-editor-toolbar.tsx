import { Code2, LayoutGrid, Maximize, Save, Share2, ZoomIn, ZoomOut } from "lucide-react";
import { useReactFlow, useViewport } from "@xyflow/react";
import type { ReactNode } from "react";

import { Button, PillGroup, type PillGroupItem } from "@compozy/ui";

import type { LoopEditorView } from "../../hooks/use-loop-editor-state";

interface LoopEditorToolbarProps {
  busy: boolean;
  positionsDirty: boolean;
  view: LoopEditorView;
  onViewChange: (view: LoopEditorView) => void;
  onAutoLayout: () => void;
  onSaveLayout: () => void;
}

const VIEW_ITEMS: PillGroupItem<LoopEditorView>[] = [
  {
    value: "graph",
    label: (
      <span className="flex items-center gap-1.5">
        <Share2 aria-hidden="true" className="size-3.5" />
        Graph
      </span>
    ),
    testId: "loop-editor-view-graph",
  },
  {
    value: "dsl",
    label: (
      <span className="flex items-center gap-1.5">
        <Code2 aria-hidden="true" className="size-3.5" />
        DSL
      </span>
    ),
    testId: "loop-editor-view-dsl",
  },
];

/**
 * Canvas sub-toolbar: zoom/auto-layout, Graph|DSL toggle, and Save layout. Editor actions live
 * in the shell topbar via `useTopbarSlot`; the read-only/fork notice is owned by
 * `LoopEditorSourceStrip` (one notice, not two). Invariant detail lives in the Validation dock
 * — not duplicated as toolbar chips.
 *
 * Layout stays available on a read-only definition: positions are the annotations sidecar, and
 * `PutLoopAnnotations` has no source gate — withholding it would invent a restriction the
 * runtime does not impose.
 */
export function LoopEditorToolbar({
  busy,
  positionsDirty,
  view,
  onViewChange,
  onAutoLayout,
  onSaveLayout,
}: LoopEditorToolbarProps) {
  const flow = useReactFlow();
  const { zoom } = useViewport();

  return (
    <div className="flex min-h-12 flex-none items-center gap-2.5 border-b border-line bg-canvas-soft px-3.5">
      <PillGroup
        aria-label="Editor view"
        className="[&_[data-slot=pill-group-item]]:min-h-11"
        items={VIEW_ITEMS}
        onChange={onViewChange}
        size="sm"
        value={view}
      />

      <div className="ml-auto flex items-center gap-2.5">
        <div className="flex items-center gap-0.5 rounded-md border border-line-soft bg-input-fill p-0.5">
          <ToolIcon label="Auto layout" onClick={onAutoLayout}>
            <LayoutGrid aria-hidden="true" className="size-3.5" />
          </ToolIcon>
          <ToolIcon label="Zoom out" onClick={() => void flow.zoomOut()}>
            <ZoomOut aria-hidden="true" className="size-3.5" />
          </ToolIcon>
          <span className="min-w-[38px] px-1 text-center font-mono text-mono-id text-subtle">
            {Math.round(zoom * 100)}%
          </span>
          <ToolIcon label="Zoom in" onClick={() => void flow.zoomIn()}>
            <ZoomIn aria-hidden="true" className="size-3.5" />
          </ToolIcon>
          <ToolIcon label="Fit view" onClick={() => void flow.fitView()}>
            <Maximize aria-hidden="true" className="size-3.5" />
          </ToolIcon>
        </div>

        <Button
          data-testid="loop-editor-save"
          disabled={busy || !positionsDirty}
          onClick={onSaveLayout}
          size="sm"
          title="Persist node positions to the annotations sidecar. Structural edits publish through Publish."
          type="button"
          variant="ghost"
        >
          <Save aria-hidden="true" className="size-3.5" />
          Save layout
        </Button>
      </div>
    </div>
  );
}

function ToolIcon({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <Button
      className="size-6 text-muted hover:bg-elevated hover:text-fg-strong"
      onClick={onClick}
      size="icon-xs"
      title={label}
      aria-label={label}
      type="button"
      variant="ghost"
    >
      {children}
    </Button>
  );
}
