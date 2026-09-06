import type { SessionRow } from "./session-timeline.logic";

/*
 * Which rows hold a given projected part (`partIndex`), so a find jump can open
 * exactly the disclosures between the reader and the matched field: the turn
 * fold, the work group, the tool row's body, the reasoning body.
 */

export function rowContainsPart(row: SessionRow, partIndex: number): boolean {
  switch (row.kind) {
    case "text":
      return row.part.partIndex === partIndex;
    case "reasoning":
      return row.parts.some(part => part.partIndex === partIndex);
    case "data":
      return row.parts.some(part => part.partIndex === partIndex);
    case "work":
    case "live-tool":
      return row.entries.some(entry => entry.partIndex === partIndex);
    case "turn-fold":
      return rowsContainPart(row.rows, partIndex);
    case "working":
    case "changed-files":
      return false;
  }
}

export function rowsContainPart(rows: readonly SessionRow[], partIndex: number): boolean {
  return rows.some(row => rowContainsPart(row, partIndex));
}
