/**
 * Copy for the session-list toolbar toggles. Kept apart from
 * `GLOBAL_SCOPE_COPY`: that globe owns where new work is created, these own how
 * the session list reads — how wide, and whether it shows the archive.
 */
export const SESSION_SCOPE_COPY = {
  controlName: "All workspaces",
  tooltipOn: "Showing every workspace",
  tooltipOff: "Showing this workspace",
  liveOn: "Sessions from every workspace",
  liveOff: "Sessions from this workspace",
} as const;

export const SESSION_ARCHIVED_COPY = {
  controlName: "Archived",
  tooltipOn: "Showing archived sessions",
  tooltipOff: "Showing active sessions",
  liveOn: "Archived sessions",
  liveOff: "Active sessions",
} as const;
