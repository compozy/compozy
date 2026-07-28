/**
 * Shared per-day chart axis chrome so the area and stacked-bar charts keep one
 * endpoint-label behaviour and one viz typography source.
 */

export const DAY_CHART_AXIS_LINE = { stroke: "var(--color-line-strong)" };

export const DAY_CHART_TICK = {
  fill: "var(--color-faint)",
  fontFamily: "var(--font-mono)",
  fontSize: "var(--text-micro)",
};

/** Stacked day-bar corner radius; mirrors `--radius-xxs` (recharts needs px). */
export const DAY_CHART_BAR_RADIUS: [number, number, number, number] = [3, 3, 0, 0];

/** Labels only the first and last day so the axis stays uncluttered. */
export function dayChartEndpointTickFormatter(dataLength: number) {
  return (value: string, index: number): string =>
    index === 0 || index === dataLength - 1 ? value : "";
}
