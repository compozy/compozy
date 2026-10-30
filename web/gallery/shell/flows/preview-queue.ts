/**
 * Which flow nodes mount their live preview. Each preview boots a whole prototype app, so the
 * canvas plans them only from a settled viewport: nodes on screen at preview zoom start loading,
 * nearest to the center first and at most `maxLoadingPreviews` at a time; a loaded preview stays
 * mounted while it remains near the viewport, up to `maxKeptPreviews`, so panning back does not
 * reload it. Pure, so the queue is testable without a browser.
 */
import { previewZoom } from "#gallery/shell/flows/edge-style";

export const maxLoadingPreviews = 3;
export const maxKeptPreviews = 12;

export type PreviewEntry = { id: string; src: string; status: "loading" | "loaded" };
/** Mounted previews, least recently seen first. */
export type PreviewPlan = readonly PreviewEntry[];

/** Node ids on screen at preview zoom (highest priority first) and ids near enough to keep. */
export type PreviewView = { visible: readonly string[]; near: ReadonlySet<string> };

export type NodeBox = { id: string; x: number; y: number; width: number; height: number };
export type ScreenViewport = { x: number; y: number; zoom: number; width: number; height: number };

/** Screen margin, in px, within which a node counts as on screen. */
const visibleMargin = 120;

/** The candidates of a settled viewport; loaded previews within one screen of it are kept. */
export function previewView(nodes: readonly NodeBox[], viewport: ScreenViewport): PreviewView {
  const { x, y, zoom, width, height } = viewport;
  const centerX = width / 2;
  const centerY = height / 2;
  const visible: { id: string; distance: number }[] = [];
  const near = new Set<string>();
  for (const node of nodes) {
    const left = node.x * zoom + x;
    const top = node.y * zoom + y;
    const right = left + node.width * zoom;
    const bottom = top + node.height * zoom;
    const within = (marginX: number, marginY: number) =>
      right > -marginX && bottom > -marginY && left < width + marginX && top < height + marginY;
    if (within(width, height)) near.add(node.id);
    if (zoom >= previewZoom && within(visibleMargin, visibleMargin)) {
      const distance = Math.hypot((left + right) / 2 - centerX, (top + bottom) / 2 - centerY);
      visible.push({ id: node.id, distance });
    }
  }
  visible.sort((a, b) => a.distance - b.distance);
  return { visible: visible.map(item => item.id), near };
}

/**
 * The plan for a settled view. Loading previews that left the screen are cancelled and loaded
 * ones that left its surroundings unmount; a preview whose source changed (theme) starts over.
 * New previews start while fewer than `maxLoadingPreviews` load, replacing the least recently
 * seen loaded preview off screen once `maxKeptPreviews` are mounted.
 */
export function schedulePreviews(
  plan: PreviewPlan,
  view: PreviewView,
  sources: ReadonlyMap<string, string>
): PreviewPlan {
  const visible = new Set(view.visible);
  const kept = plan.filter(entry =>
    sources.get(entry.id) !== entry.src
      ? false
      : entry.status === "loading"
        ? visible.has(entry.id)
        : view.near.has(entry.id)
  );
  // Previews on screen become the most recently seen, in priority order.
  const next = [
    ...kept.filter(entry => !visible.has(entry.id)),
    ...view.visible.flatMap(id => kept.filter(entry => entry.id === id)),
  ];
  let loading = next.filter(entry => entry.status === "loading").length;
  for (const id of view.visible) {
    if (loading >= maxLoadingPreviews) break;
    const src = sources.get(id);
    if (src === undefined || next.some(entry => entry.id === id)) continue;
    if (next.length >= maxKeptPreviews) {
      const evicted = next.findIndex(entry => entry.status === "loaded" && !visible.has(entry.id));
      if (evicted < 0) break;
      next.splice(evicted, 1);
    }
    next.push({ id, src, status: "loading" });
    loading++;
  }
  return next;
}

/** A preview that reported ready or failed frees its loading slot; it stays mounted. */
export function settlePreview(plan: PreviewPlan, id: string, src: string): PreviewPlan {
  return plan.map(entry =>
    entry.id === id && entry.src === src && entry.status === "loading"
      ? { ...entry, status: "loaded" }
      : entry
  );
}
