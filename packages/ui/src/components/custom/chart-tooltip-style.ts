import type * as React from "react";

/** Shared overlay styling for recharts tooltips — the popover surface (canvas + pop shadow). */
export const CHART_TOOLTIP_CONTENT_STYLE: React.CSSProperties = {
  background: "var(--color-canvas)",
  border: "1px solid var(--color-line)",
  borderRadius: "var(--radius-md)",
  boxShadow: "var(--shadow-pop)",
  color: "var(--color-muted)",
  fontFamily: "var(--font-mono)",
  fontSize: "var(--text-micro)",
  padding: "var(--space-chart-tooltip-y) var(--space-chart-tooltip-x)",
};
