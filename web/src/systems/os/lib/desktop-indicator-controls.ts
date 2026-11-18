export interface DesktopIndicatorItem {
  id: string;
  name: string;
  /** A window on this desktop needs you; shown only while it is off-screen. */
  needsYou?: boolean;
}

export type DesktopOverflowDirection = "earlier" | "later";

/** Marks an overflow control so a tab drag can find it under the pointer; its value is the side. */
export const DESKTOP_OVERFLOW_ATTRIBUTE = "data-desktop-overflow";

export interface DesktopOverflowRequest {
  direction: DesktopOverflowDirection;
  hiddenDesktopIds: readonly string[];
  anchorDesktopId: string;
}

export interface DesktopIndicatorControl {
  kind: "desktop";
  key: string;
  desktop: DesktopIndicatorItem;
  /** The desktop's position among every desktop, not only the shown ones. */
  position: number;
}

export interface DesktopOverflowControl {
  kind: "overflow";
  key: string;
  direction: DesktopOverflowDirection;
  hiddenDesktopIds: readonly string[];
}

export type DesktopBarControl = DesktopIndicatorControl | DesktopOverflowControl;

const ALL_DESKTOPS_LIMIT = 7;
const WINDOW_RADIUS = 2;
const WINDOW_SIZE = WINDOW_RADIUS * 2 + 1;

/**
 * The top bar's controls: every desktop through seven; past that, five indicators around the
 * active desktop with an overflow control on each side that hides desktops.
 */
export function desktopBarControls(
  desktops: readonly DesktopIndicatorItem[],
  activeDesktopId: string
): DesktopBarControl[] {
  if (desktops.length <= ALL_DESKTOPS_LIMIT) {
    return desktops.map((desktop, position) => ({
      kind: "desktop",
      key: `desktop:${desktop.id}`,
      desktop,
      position,
    }));
  }

  const activePosition = Math.max(
    0,
    desktops.findIndex(desktop => desktop.id === activeDesktopId)
  );
  const start = Math.min(
    Math.max(activePosition - WINDOW_RADIUS, 0),
    desktops.length - WINDOW_SIZE
  );
  const end = start + WINDOW_SIZE;
  const controls: DesktopBarControl[] = [];
  const earlier = desktops.slice(0, start);
  const later = desktops.slice(end);

  if (earlier.length > 0) {
    controls.push({
      kind: "overflow",
      key: "overflow:earlier",
      direction: "earlier",
      hiddenDesktopIds: earlier.map(desktop => desktop.id),
    });
  }
  desktops.slice(start, end).forEach((desktop, offset) => {
    controls.push({
      kind: "desktop",
      key: `desktop:${desktop.id}`,
      desktop,
      position: start + offset,
    });
  });
  if (later.length > 0) {
    controls.push({
      kind: "overflow",
      key: "overflow:later",
      direction: "later",
      hiddenDesktopIds: later.map(desktop => desktop.id),
    });
  }
  return controls;
}

/** The overview request an overflow control makes: open on the nearest hidden desktop. */
export function desktopOverflowRequest(
  controls: readonly DesktopBarControl[],
  direction: DesktopOverflowDirection,
  anchorDesktopId: string
): DesktopOverflowRequest | null {
  const control = controls.find(
    (candidate): candidate is DesktopOverflowControl =>
      candidate.kind === "overflow" && candidate.direction === direction
  );
  if (control === undefined) return null;
  return { direction, hiddenDesktopIds: control.hiddenDesktopIds, anchorDesktopId };
}

export function desktopOverflowLabel(control: DesktopOverflowControl): string {
  const count = control.hiddenDesktopIds.length;
  return `Show ${count} ${control.direction} desktop${count === 1 ? "" : "s"}`;
}

export function desktopIndicatorLabel(
  control: DesktopIndicatorControl,
  total: number,
  active: boolean
): string {
  const base = `Desktop ${control.position + 1} of ${total}: ${control.desktop.name}`;
  return control.desktop.needsYou && !active ? `${base} — needs you` : base;
}
