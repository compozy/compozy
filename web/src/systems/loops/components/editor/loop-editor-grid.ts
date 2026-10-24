import type { LoopEditorPaletteMode } from "../../lib/loop-editor-types";

const EDITOR_GRID_CLASS = {
  "rail:inspector":
    "grid-cols-[190px_minmax(0,1fr)_320px] xl:grid-cols-[210px_minmax(0,1fr)_344px]",
  "rail:canvas": "grid-cols-[190px_minmax(0,1fr)] xl:grid-cols-[210px_minmax(0,1fr)]",
  "none:inspector": "grid-cols-[minmax(0,1fr)_320px] xl:grid-cols-[minmax(0,1fr)_344px]",
  "none:canvas": "grid-cols-[minmax(0,1fr)]",
} as const;

/** The editor's column split, shared by the ready editor and its skeleton. */
export function editorGridClass(
  paletteMode: LoopEditorPaletteMode,
  inspectorOpen: boolean
): string {
  const rail = paletteMode === "expanded" ? "rail" : "none";
  return EDITOR_GRID_CLASS[`${rail}:${inspectorOpen ? "inspector" : "canvas"}`];
}
