/**
 * The roster's closed column set; a further column is a design decision, not a
 * typo. Project is the All projects listing's identity column (`_uiux.md` S5).
 */
export type LoopRunColumnKey = "loop" | "project" | "status" | "progress" | "started" | "duration";

export interface LoopRunColumn {
  key: LoopRunColumnKey;
  label: string;
  className?: string;
}

/** Gens / Best / Budget are demoted to the run page. */
export const COLUMNS: readonly LoopRunColumn[] = [
  { key: "loop", label: "Loop", className: "w-full" },
  { key: "status", label: "Status" },
  { key: "progress", label: "Progress" },
  { key: "started", label: "Started" },
  { key: "duration", label: "Duration" },
];

/** The All projects listing's identity column, after the Loop. */
export const PROJECT_COLUMN: LoopRunColumn = { key: "project", label: "Project" };

/** The columns a roster prints: Project joins only under All projects. */
export function rosterColumns(withProject: boolean): readonly LoopRunColumn[] {
  return withProject ? [COLUMNS[0], PROJECT_COLUMN, ...COLUMNS.slice(1)] : COLUMNS;
}
