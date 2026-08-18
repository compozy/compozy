import {
  Code2,
  LayoutGrid,
  Maximize,
  PanelLeft,
  PanelRight,
  Save,
  Share2,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import { useReactFlow, useViewport } from "@xyflow/react";
import type { ReactNode } from "react";

import { Button, cn, PillGroup, type PillGroupItem } from "@compozy/ui";

import type { LoopEditorView } from "../../hooks/use-loop-editor-state";
import type { PaletteItem } from "../../lib/loop-palette";
import type { LoopEditorPaletteMode } from "./loop-editor-palette";
import { LoopEditorPaletteMenu } from "./loop-editor-palette-menu";

interface LoopEditorToolbarProps {
  busy: boolean;
  positionsDirty: boolean;
  view: LoopEditorView;
  onViewChange: (view: LoopEditorView) => void;
  onAutoLayout: () => void;
  onSaveLayout: () => void;
  onAddNode: (item: PaletteItem) => void;
  addNodeDisabled?: boolean;

  paletteMode: LoopEditorPaletteMode;
  onTogglePalette: () => void;
  inspectorOpen: boolean;
  onToggleInspector: () => void;
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

export function LoopEditorToolbar({
  busy,
  positionsDirty,
  view,
  onViewChange,
  onAutoLayout,
  onSaveLayout,
  onAddNode,
  addNodeDisabled = false,
  paletteMode,
  onTogglePalette,
  inspectorOpen,
  onToggleInspector,
}: LoopEditorToolbarProps) {
  const flow = useReactFlow();
  const { zoom } = useViewport();
  const paletteExpanded = paletteMode === "expanded";

  return (
    <div className="flex min-h-12 flex-none items-center gap-2.5 border-b border-line bg-canvas-soft px-3.5">
      {paletteMode === "menu" ? null : (
        <Button
          aria-label={paletteExpanded ? "Close node palette" : "Open node palette"}
          aria-pressed={paletteExpanded}
          className={cn(paletteExpanded ? "bg-elevated text-fg" : null)}
          data-state={paletteExpanded ? "open" : "closed"}
          data-testid="loop-editor-palette-toggle"
          onClick={onTogglePalette}
          size="icon-sm"
          type="button"
          variant="ghost"
        >
          <PanelLeft aria-hidden="true" className="size-4" />
        </Button>
      )}

      <PillGroup
        aria-label="Editor view"
        className="[&_[data-slot=pill-group-item]]:min-h-11"
        items={VIEW_ITEMS}
        onChange={onViewChange}
        size="sm"
        value={view}
      />

      {paletteExpanded ? null : (
        <LoopEditorPaletteMenu disabled={addNodeDisabled} onAddNode={onAddNode} />
      )}

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

        <Button
          aria-label={inspectorOpen ? "Close inspector" : "Open inspector"}
          aria-pressed={inspectorOpen}
          className={cn(inspectorOpen ? "bg-elevated text-fg" : null)}
          data-state={inspectorOpen ? "open" : "closed"}
          data-testid="loop-editor-inspector-toggle"
          onClick={onToggleInspector}
          size="icon-sm"
          type="button"
          variant="ghost"
        >
          <PanelRight aria-hidden="true" className="size-4" />
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
