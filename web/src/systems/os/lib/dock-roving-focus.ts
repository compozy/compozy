const DOCK_ITEM_SELECTOR = '[data-slot="os-dock-item"]:not(:disabled)';

const STEP_KEYS = {
  vertical: { next: "ArrowDown", previous: "ArrowUp" },
  horizontal: { next: "ArrowRight", previous: "ArrowLeft" },
} as const;

/**
 * Roving focus across the dock's launchers (rail: Up/Down, compact tab bar:
 * Left/Right), wrapping at the ends; Home/End jump.
 */
export function moveDockRovingFocus(
  event: React.KeyboardEvent<HTMLElement>,
  orientation: keyof typeof STEP_KEYS
): void {
  const items = Array.from(event.currentTarget.querySelectorAll<HTMLElement>(DOCK_ITEM_SELECTOR));
  const index = items.findIndex(item => item === event.target);
  const keys = STEP_KEYS[orientation];
  const targets: Record<string, number> = {
    [keys.next]: index + 1,
    [keys.previous]: index - 1,
    Home: 0,
    End: items.length - 1,
  };
  const target = targets[event.key];
  if (index < 0 || target === undefined) return;
  event.preventDefault();
  items[(target + items.length) % items.length]?.focus();
}

/** One tab stop per launcher strip: the last focused, else the focused app's, else the first. */
export function dockTabStopId(
  items: readonly { id: string; active?: boolean }[],
  lastFocusedId: string | null
): string | undefined {
  if (lastFocusedId !== null && items.some(item => item.id === lastFocusedId)) return lastFocusedId;
  return items.find(item => item.active)?.id ?? items[0]?.id;
}
