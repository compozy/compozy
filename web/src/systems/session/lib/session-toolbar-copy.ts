/**
 * Copy for the session-list toolbar toggles. Breadth is not here: the topbar
 * globe owns it for every list (ADR-004); this owns whether the list shows the
 * archive or the sessions parked in Later.
 */
export const SESSION_ARCHIVED_COPY = {
  controlName: "Archived",
  tooltipOn: "Showing archived sessions",
  tooltipOff: "Showing active sessions",
  liveOn: "Archived sessions",
  liveOff: "Active sessions",
} as const;

export const SESSION_LATER_COPY = {
  controlName: "Later",
  tooltipOn: "Showing sessions in Later",
  tooltipOff: "Later",
  liveOn: "Sessions in Later",
  liveOff: "Active sessions",
} as const;
