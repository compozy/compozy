/**
 * True when a global shortcut should yield to the focused element — dock digit
 * shortcuts ignore inputs, textareas, selects, and contenteditable surfaces.
 */
export function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable) return true;
  const tag = target.tagName;
  return tag === "TEXTAREA" || tag === "INPUT" || tag === "SELECT";
}
