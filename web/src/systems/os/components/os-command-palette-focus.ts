/** Restores keyboard entry on the visible stacked frame after a pop unmounts the child. */
export function focusActivePaletteFrame(frame: HTMLElement | null): void {
  const input = frame?.querySelector<HTMLElement>("[data-slot='command-input']");
  input?.focus();
}
