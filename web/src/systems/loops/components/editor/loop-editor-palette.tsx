import { Eyebrow, KindIcon, type KindIconRegistry } from "@compozy/ui";

import {
  LOOP_CALL_TOOL_ICON,
  LOOP_NODE_KIND_ICONS,
  loopNodeClassIcon,
} from "../../lib/loop-node-kind-icons";
import { LOOP_PALETTE, type PaletteItem } from "../../lib/loop-palette";
import { MonoTag } from "../mono-tag";

interface LoopEditorPaletteProps {
  onAddNode: (item: PaletteItem) => void;
  disabled?: boolean;
}

const LOOP_EDITOR_KIND_ICON_REGISTRY = {
  ...LOOP_NODE_KIND_ICONS,
  "": LOOP_CALL_TOOL_ICON,
} satisfies KindIconRegistry;

function paletteKindKey(kindLabel: string): string {
  return kindLabel === "tool…" ? "" : kindLabel;
}

/**
 * The "Add node" palette (design §4.6), grouped by class. Fork-and-edit only: it extends
 * the open definition's body — there is no blank-canvas builder (ADR-008). Beyond the
 * three reserved action kinds it lists a "Channel post" shortcut and a "Call tool…"
 * entry; an action's kind IS the ToolID it calls (ADR-021). Click adds the node to the
 * canvas; the author then connects it (the linter flags reachability until wired).
 */
export function LoopEditorPalette({ onAddNode, disabled = false }: LoopEditorPaletteProps) {
  return (
    <aside
      className="hidden min-h-0 flex-col gap-4 overflow-y-auto border-r border-line bg-canvas px-3 py-3.5 lg:flex"
      data-testid="loop-editor-palette"
    >
      <Eyebrow className="text-subtle">Add node</Eyebrow>
      {LOOP_PALETTE.map(group => (
        <div key={group.label} className="flex flex-col gap-1.5">
          <MonoTag className="px-0.5 text-pill-group-badge tracking-[0.09em] text-faint">
            {group.label}
          </MonoTag>
          {group.items.map(item => (
            <button
              key={item.label}
              type="button"
              onClick={() => onAddNode(item)}
              disabled={disabled}
              title={item.hint}
              data-testid={`loop-palette-item-${item.kindLabel}`}
              className="group flex items-center gap-2 rounded-md border border-line-soft bg-canvas-soft px-2 py-1.5 text-left transition-colors hover:border-line-strong hover:bg-canvas-tint disabled:cursor-not-allowed disabled:opacity-60"
            >
              <span className="grid size-4 shrink-0 place-items-center rounded-xs bg-badge-fill transition-transform group-hover:scale-110">
                <KindIcon
                  className="size-2.5"
                  fallback={loopNodeClassIcon({
                    nodeClass: item.nodeClass,
                    isFanOut: item.kindLabel === "fan-out",
                    isGate: item.kindLabel === "gate",
                  })}
                  kind={paletteKindKey(item.kindLabel)}
                  registry={LOOP_EDITOR_KIND_ICON_REGISTRY}
                  size="xs"
                  tone="muted"
                />
              </span>
              <span className="truncate text-form-label font-medium text-fg-strong">
                {item.label}
              </span>
            </button>
          ))}
        </div>
      ))}
    </aside>
  );
}
