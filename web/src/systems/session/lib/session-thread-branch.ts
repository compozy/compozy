/** The two kinds of descendants a session row can unfold beneath it. */
export type SessionThreadSegment = "children" | "subagents";

/**
 * Which list a row's branch shows. Child sessions start unfolded (a collapse
 * is remembered); the subagent list starts closed (an open is remembered), so
 * no roster is read until the operator asks for it. At most one list shows:
 * opening subagents also folds the children.
 */
export function sessionThreadSegment(input: {
  hasChildren: boolean;
  hasSubagents: boolean;
  collapsed: boolean;
  subagentsOpen: boolean;
}): SessionThreadSegment | null {
  if (input.hasSubagents && input.subagentsOpen) return "subagents";
  if (input.hasChildren && !input.collapsed) return "children";
  return null;
}

/**
 * The branch after a click: the chevron opens the first list there is or
 * folds the open one; a segment opens its list, or folds it when it is the
 * one already showing. `null` folds the branch.
 */
export function nextSessionThreadSegment(
  current: SessionThreadSegment | null,
  clicked: SessionThreadSegment | "chevron",
  hasChildren: boolean
): SessionThreadSegment | null {
  if (clicked === "chevron") {
    if (current !== null) return null;
    return hasChildren ? "children" : "subagents";
  }
  return clicked === current ? null : clicked;
}

/** Shows one list under a session row, or folds its branch (`null`). */
export type SessionThreadBranchChange = (
  sessionId: string,
  segment: SessionThreadSegment | null
) => void;
