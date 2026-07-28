/**
 * Rows, columns and a stack, drawn rather than named. The daemon's word for the
 * first two is an axis whose meaning is the opposite of what it sounds like, so
 * the picture is the label and `window-manager-layout-mutations.ts` owns the
 * translation.
 */
const SHARED = {
  "aria-hidden": true,
  fill: "none",
  viewBox: "0 0 26 17",
} as const;

const PANE = {
  fill: "currentColor",
  fillOpacity: 0.22,
  stroke: "currentColor",
  strokeOpacity: 0.7,
} as const;

export function LayoutRowsDiagram({ className }: { className?: string }) {
  return (
    <svg {...SHARED} className={className}>
      <rect {...PANE} height="7" rx="1.4" width="24.6" x="0.7" y="0.7" />
      <rect {...PANE} height="7" rx="1.4" width="24.6" x="0.7" y="9.3" />
    </svg>
  );
}

export function LayoutColumnsDiagram({ className }: { className?: string }) {
  return (
    <svg {...SHARED} className={className}>
      <rect {...PANE} height="15.6" rx="1.4" width="11.6" x="0.7" y="0.7" />
      <rect {...PANE} height="15.6" rx="1.4" width="11.6" x="13.7" y="0.7" />
    </svg>
  );
}

export function LayoutStackDiagram({ className }: { className?: string }) {
  return (
    <svg {...SHARED} className={className}>
      <rect
        fill="currentColor"
        fillOpacity="0.12"
        height="12.6"
        rx="1.4"
        stroke="currentColor"
        strokeOpacity="0.4"
        width="19"
        x="0.7"
        y="3.7"
      />
      <rect
        fill="currentColor"
        fillOpacity="0.18"
        height="12.6"
        rx="1.4"
        stroke="currentColor"
        strokeOpacity="0.55"
        width="19"
        x="3.7"
        y="2.2"
      />
      <rect
        fill="currentColor"
        fillOpacity="0.26"
        height="12.6"
        rx="1.4"
        stroke="currentColor"
        strokeOpacity="0.85"
        width="19"
        x="6.3"
        y="0.7"
      />
    </svg>
  );
}
