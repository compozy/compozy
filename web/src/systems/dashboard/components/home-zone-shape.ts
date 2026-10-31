// The frames and static copy the live Home zones and their placeholders share, so a zone in its
// loading shape sits exactly where the loaded one will. Plain constants with no imports: the
// placeholders (`home-zone-skeletons.tsx`) stay free of hooks, queries and routing.

/** The Home body's vertical padding inside the page gutter. */
export const HOME_BODY_CLASS = "pt-6 pb-20";

/** The stack every zone sits in. */
export const HOME_ZONE_STACK_CLASS = "flex min-w-0 flex-col gap-6";

/** The two-up grid the paired zones (outcomes | usage, agents | activity) share. */
export const HOME_ZONE_PAIR_CLASS = "grid grid-cols-1 items-stretch gap-5 min-[1080px]:grid-cols-2";

/** The KPI tile labels. */
export const HOME_KPI_LABELS = {
  workingNow: "Working now",
  needsYou: "Needs you",
  completedToday: "Completed today",
} as const;

/** The usage tile's sparkline height; its placeholder reserves the taller head row it makes. */
export const HOME_KPI_SPARKLINE_HEIGHT = 22;

export function homeUsageLabel(windowDays: number): string {
  return `Usage, last ${windowDays} days`;
}

/** One live-work row: its grid and its two lines. */
export const HOME_RUN_ROW_CLASS =
  "grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-4 py-3";
export const HOME_RUN_TITLE_CLASS = "flex items-center gap-2 text-body font-medium text-fg-strong";
export const HOME_RUN_SUBTITLE_CLASS =
  "mt-1 flex min-w-0 items-center gap-1.5 text-small-body text-muted-foreground";

/** The pulse heatmap: weekday rows over 24 hour cells, plus its insights line. */
export const HOME_PULSE_WEEKDAY_LABELS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"] as const;
/** Monday-first display order over the backend's Sunday-first weekday index. */
export const HOME_PULSE_WEEKDAY_ROWS = [1, 2, 3, 4, 5, 6, 0] as const;
/** Fixed weekday-label track + 24 hour cells, sized by the home-pulse tokens. */
export const HOME_PULSE_GRID_COLUMNS =
  "var(--size-home-pulse-label) repeat(24, minmax(var(--size-home-pulse-cell-min), 1fr))";
export const HOME_PULSE_GRID_CLASS =
  "grid min-w-[var(--size-home-pulse-min-w)] gap-[var(--space-home-pulse-gap)]";
export const HOME_PULSE_INSIGHTS_CLASS =
  "flex flex-wrap items-center gap-2.5 border-t border-line-soft px-4 py-2.5 text-small-body text-muted-foreground";

/** The outcomes and usage charts' headline rows and the outcomes legend. */
export const HOME_OUTCOMES_HEADLINE_CLASS = "flex items-baseline gap-2.5";
export const HOME_OUTCOMES_LEGEND_CLASS =
  "flex flex-wrap items-center gap-4 text-small-body text-muted-foreground";
export const HOME_USAGE_HEADLINE_CLASS = "flex items-baseline gap-2.5";

/** The agents table: column grid (with its narrow form), row frame and name cell. */
export const HOME_AGENTS_ROW_GRID = "grid-cols-[minmax(0,1.1fr)_52px_64px_52px_minmax(0,1fr)]";
const HOME_AGENTS_ROW_GRID_NARROW = "max-[760px]:grid-cols-[minmax(0,1fr)_52px_minmax(0,1fr)]";
export const HOME_AGENTS_ROW_CLASS = `grid ${HOME_AGENTS_ROW_GRID} ${HOME_AGENTS_ROW_GRID_NARROW} flex-1 items-center gap-3 px-4 py-2.5`;
export const HOME_AGENTS_NAME_CLASS =
  "flex min-w-0 items-center gap-2 text-body font-medium text-fg-strong";

/** One activity row: marker, line, time. */
export const HOME_ACTIVITY_ROW_CLASS =
  "grid grid-cols-[14px_minmax(0,1fr)_auto] items-start gap-2.5 px-4 py-2.5";

/** The folded system row's frame. */
export const HOME_SYSTEM_ROW_CLASS = "flex w-full items-center gap-2.5 px-4 py-3";
