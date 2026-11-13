/**
 * Shortcut IDs the window manager retired with tabs and splits. A stored
 * binding to one still loads and does nothing (US-018.EC-1); the daemon skips
 * them (`internal/windowmanager/retired_shortcuts.go`) but its settings payload
 * does not say which IDs are retired, so Settings keeps this one list to mark
 * them. Remove in v0.8.0, together with the daemon's acceptance of these IDs.
 */
const RETIRED_SHORTCUT_TITLES: Readonly<Record<string, string>> = {
  "window.minimize": "Minimize window",
  "window.zoom": "Zoom window",
  "window.toggle_floating": "Float window",
  "window.tile.top-left": "Tile to top left",
  "window.tile.top-right": "Tile to top right",
  "window.tile.bottom-left": "Tile to bottom left",
  "window.tile.bottom-right": "Tile to bottom right",
  "window.merge_all": "Merge all windows",
  "window.tab.detach": "Detach tab",
};

export function isRetiredShortcutId(commandId: string): boolean {
  return Object.hasOwn(RETIRED_SHORTCUT_TITLES, commandId);
}

/** The name the action had before it was retired, so the row reads as what the person bound. */
export function retiredShortcutTitle(commandId: string): string {
  return RETIRED_SHORTCUT_TITLES[commandId] ?? commandId;
}

/**
 * The map without its retired entries. The daemon skips a retired binding, so
 * its chord is free: it must not block or shadow a live action in any conflict
 * check, even while the row stays listed for removal.
 */
export function withoutRetiredShortcuts<T>(map: Readonly<Record<string, T>>): Record<string, T> {
  return Object.fromEntries(Object.entries(map).filter(([id]) => !isRetiredShortcutId(id)));
}
