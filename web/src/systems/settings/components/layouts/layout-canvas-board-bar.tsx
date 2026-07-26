import { Pill } from "@agh/ui";

import type { LayoutProjection, LayoutProjectionDiagnostic } from "@/systems/os";

import { LAYOUT_CANVAS_REFERENCE } from "../../lib/window-manager-layout-reference";
import { layoutWindowFace } from "../../lib/window-manager-layout-window-face";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
} from "../../lib/window-manager-layout-types";

interface LayoutCanvasBoardBarProps {
  document: WindowManagerLayoutDocument;
  desktop: WindowManagerLayoutDesktop;
  projection: LayoutProjection;
}

const REFERENCE_LABEL = `${LAYOUT_CANVAS_REFERENCE.w} × ${LAYOUT_CANVAS_REFERENCE.h}`;

/**
 * What is on this desktop, and — because the canvas is a fixed reference screen
 * rather than the viewport in front of you — the size that reading is true at.
 * Without that caption the fit warnings below would be a claim about a screen
 * nobody named.
 */
export function LayoutCanvasBoardBar({ document, desktop, projection }: LayoutCanvasBoardBarProps) {
  const floating = desktop.floating.filter(id => document.windows[id]?.minimized === false).length;
  const minimized = desktop.floating.filter(id => document.windows[id]?.minimized === true).length;
  const focusOwner = desktop.focusOwner ? document.windows[desktop.focusOwner] : undefined;
  const notes = fitNotes(document, projection.diagnostics);

  return (
    <div className="flex flex-col gap-1.5" data-testid="layout-canvas-board-bar">
      <div className="flex min-h-5 flex-wrap items-center gap-x-3 gap-y-1 text-form-hint text-subtle">
        <span>{projection.windows.length} tiled</span>
        <span>{floating} floating</span>
        {minimized > 0 ? <span>{minimized} minimized</span> : null}
        <span className="flex-1" />
        {desktop.purpose === "focus" ? (
          <Pill size="xs" tone="info">
            Focus desktop
            {focusOwner ? ` · ${layoutWindowFace(focusOwner, focusOwner.id).title}` : ""}
          </Pill>
        ) : null}
        <span className="font-mono text-badge text-faint">{REFERENCE_LABEL} reference</span>
      </div>
      {notes.length > 0 ? (
        <ul
          className="flex flex-col gap-0.5 text-form-hint text-muted"
          data-testid="layout-canvas-fit-notes"
          role="status"
        >
          {notes.map(note => (
            <li key={note}>{note}</li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

function fitNotes(
  document: WindowManagerLayoutDocument,
  diagnostics: readonly LayoutProjectionDiagnostic[]
): string[] {
  const folded = diagnostics.filter(entry => entry.code === "adaptive-stack").length;
  const cramped = diagnostics.flatMap(entry =>
    entry.code === "minimum-unmet"
      ? [layoutWindowFace(document.windows[entry.windowId], entry.windowId).title]
      : []
  );
  const invalid = diagnostics.filter(entry => entry.code === "invalid-group-frame").length;

  const notes: string[] = [];
  if (folded > 0) {
    notes.push(
      `At ${REFERENCE_LABEL}, ${folded} split${folded === 1 ? "" : "s"} ${folded === 1 ? "folds" : "fold"} into a stack because the windows no longer fit side by side.`
    );
  }
  if (cramped.length > 0) {
    const unique = [...new Set(cramped)];
    notes.push(
      `At ${REFERENCE_LABEL}, ${unique.join(", ")} ${unique.length === 1 ? "is" : "are"} below the smallest size the window supports.`
    );
  }
  if (invalid > 0) {
    notes.push(
      `${invalid} group${invalid === 1 ? "" : "s"} sit outside the desktop. The daemon refuses a layout that leaves the screen.`
    );
  }
  return notes;
}
