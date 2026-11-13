/**
 * Ranges the web editor holds for the window-manager behavior it still edits.
 * The daemon is looser on gaps — it only demands finite, non-negative insets
 * (`internal/config/window_manager.go`) — so these are the editor's limits, not
 * the daemon's, and no copy on the page claims otherwise.
 */
export const WINDOW_MANAGER_RANGES = {
  gap: { min: 0, max: 64 },
  historyLimit: { min: 1, max: 500 },
} as const;
