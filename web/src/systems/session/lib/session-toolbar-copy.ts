/**
 * Copy for the session-list toolbar toggles. Breadth is not here: the topbar
 * globe owns it for every list (ADR-004); this owns whether the list shows the
 * archive.
 */
export const SESSION_ARCHIVED_COPY = {
  controlName: "Archived",
  tooltipOn: "Showing archived sessions",
  tooltipOff: "Showing active sessions",
  liveOn: "Archived sessions",
  liveOff: "Active sessions",
} as const;
