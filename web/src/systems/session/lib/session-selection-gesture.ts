/** What a click on a selectable session (a list row or a board card) does to the selection. */
export type SessionSelectionGesture = "range" | "toggle" | "open";

interface GestureKeys {
  shiftKey: boolean;
  metaKey: boolean;
  ctrlKey: boolean;
}

/**
 * Shift extends a range from the anchor; Cmd/Ctrl toggles one session; while a
 * selection is open every plain click toggles too, so only an empty selection opens.
 */
export function sessionSelectionGesture(
  event: GestureKeys,
  mode: boolean
): SessionSelectionGesture {
  if (event.shiftKey) return "range";
  if (mode || event.metaKey || event.ctrlKey) return "toggle";
  return "open";
}

/** A selection checkbox ranges on Shift (click or Space) and toggles otherwise. */
export function sessionCheckboxGesture(event: Event): Exclude<SessionSelectionGesture, "open"> {
  return "shiftKey" in event && event.shiftKey === true ? "range" : "toggle";
}
