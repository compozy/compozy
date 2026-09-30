/**
 * Default initial-focus policy shared by `DialogContent` and `SheetContent`,
 * and by `PopoverContent` when a pointer opens it.
 *
 * Opening a dialog must not paint a focus ring on its first action. Unless the
 * consumer names `initialFocus`, focus lands on the popup container itself
 * (the primitive gives it `tabIndex={-1}`), so screen readers announce the
 * dialog by its title and the next Tab moves into the controls with a normal
 * `:focus-visible` ring. A control that already took focus while mounting —
 * an `autoFocus` field in a rename or create form — keeps it.
 */
export function defaultDialogInitialFocus(popup: HTMLElement | null): HTMLElement | boolean {
  if (!popup) return true;
  const active = popup.ownerDocument.activeElement;
  if (active instanceof HTMLElement && active !== popup && popup.contains(active)) {
    return active;
  }
  return popup;
}
