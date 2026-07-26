import { GripVertical } from "lucide-react";

import { Eyebrow } from "@agh/ui";

import { LOOP_PALETTE, type PaletteItem } from "../../lib/loop-palette";
import { MonoTag } from "../mono-tag";

interface LoopEditorPaletteProps {
  onAddNode: (item: PaletteItem) => void;
  disabled?: boolean;
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
      className="flex min-h-0 flex-col gap-4 overflow-y-auto border-r border-line bg-canvas px-3 py-3.5"
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
              className="flex items-center gap-2 rounded-md border border-line-soft bg-canvas-soft px-2 py-1.5 text-left transition-colors hover:border-line-strong hover:bg-canvas-tint disabled:cursor-not-allowed disabled:opacity-60"
            >
              <GripVertical aria-hidden="true" className="size-3 shrink-0 text-faint" />
              <span className="truncate text-form-label font-medium text-fg-strong">
                {item.label}
              </span>
            </button>
          ))}
        </div>
      ))}
      <p className="rounded-md border border-line-soft bg-canvas-soft px-2.5 py-2.5 text-form-hint leading-relaxed text-faint">
        <span className="font-medium text-subtle">Kinds are ToolIDs.</span> Beyond run-agent,
        run-loop and transform, an action&apos;s kind is the tool it calls. Tool params are edited
        as JSON today; schema-driven forms are a follow-up.
      </p>
    </aside>
  );
}
