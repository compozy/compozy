// Classes shared by the live Settings nav and the window's loading fallback. In a module of their
// own so the component file stays hot-reloadable.

/** A section row's box; the live link and the static loading row both start from it. */
export const SETTINGS_NAV_ITEM_CLASS =
  "flex h-11 shrink-0 items-center gap-2.5 rounded-md px-2 text-body font-medium @min-settings-takeover:h-8";

/** The row's tone: the open section is selected, the rest recede. */
export function settingsNavItemTone(isActive: boolean): string {
  return isActive ? "bg-selected text-fg" : "text-muted";
}
