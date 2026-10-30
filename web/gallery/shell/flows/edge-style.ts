/**
 * How flow edges are drawn: kinds read by line pattern, not color, so the legend, the canvas and
 * the inspector name and draw them the same way; routes become rounded orthogonal paths; viewport
 * moves respect reduced motion.
 */
import type { EdgeKind } from "#gallery/flow";
import type { Point } from "#gallery/shell/flows/layout-state";

export const edgeKindStyle: Record<EdgeKind, { label: string; dash?: string; round?: boolean }> = {
  navigate: { label: "navigates" },
  overlay: { label: "same screen", dash: "4 4" },
  redirect: { label: "redirect", dash: "1.5 4", round: true },
  external: { label: "leaves the app", dash: "8 6" },
};

/** Line ink of edges, and of the selected node's edges. */
export const edgeInk = "var(--color-subtle)";
export const edgeInkHighlighted = "var(--color-accent)";

/** Edge label markup shared by the canvas and the hidden measurer, so ELK reserves the real size. */
export const edgeLabelClass = "whitespace-nowrap rounded-xs px-1 text-small-body font-medium";

/** At or above this zoom, prototype nodes on screen show their live preview. */
export const previewZoom = 0.6;

export function prefersReducedMotion() {
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/** Viewport moves animate 250 ms; with reduced motion they jump. */
export function viewportDuration() {
  return prefersReducedMotion() ? 0 : 250;
}

function toward(from: Point, to: Point, distance: number): Point {
  const length = Math.hypot(to.x - from.x, to.y - from.y) || 1;
  return {
    x: from.x + ((to.x - from.x) / length) * distance,
    y: from.y + ((to.y - from.y) / length) * distance,
  };
}

/** SVG path through the route points with quadratic corners of up to `radius` px. */
export function roundedPath(points: readonly Point[], radius = 8) {
  const [first, ...rest] = points;
  if (!first) return "";
  let path = `M ${first.x} ${first.y}`;
  rest.forEach((point, index) => {
    const previous = points[index] ?? first;
    const next = rest[index + 1];
    if (!next) {
      path += ` L ${point.x} ${point.y}`;
      return;
    }
    const corner = Math.min(
      radius,
      Math.hypot(point.x - previous.x, point.y - previous.y) / 2,
      Math.hypot(next.x - point.x, next.y - point.y) / 2
    );
    const start = toward(point, previous, corner);
    const end = toward(point, next, corner);
    path += ` L ${start.x} ${start.y} Q ${point.x} ${point.y} ${end.x} ${end.y}`;
  });
  return path;
}
