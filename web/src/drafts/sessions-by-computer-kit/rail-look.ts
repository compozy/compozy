import { createContext } from "react";

/** What one review state shows over the same rows: folds, the open row, a drag, a selection. */
export interface RailLook {
  /** Keys that start folded: `computer:<key>`, `project:<computer>:<ws>`, `folder:<id>`. */
  collapsed: ReadonlySet<string>;
  currentRow?: string;
  /** The row being dragged. */
  dragging?: string;
  /** The folder the drag would drop into. */
  dropInto?: string;
  /** The computer the drag hovers, which refuses it. */
  refusedOn?: string;
  selected?: ReadonlySet<string>;
  /** The filter text: folders and sections with no match hide. */
  query: string;
}

export const RailLookContext = createContext<RailLook>({ collapsed: new Set(), query: "" });
