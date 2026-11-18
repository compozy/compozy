import type { KeyboardEvent } from "react";

/**
 * The session rail's keyboard walk. Each stop is the focusable element of a
 * project-folder, group or folder header, or a session row, marked with
 * `data-sidebar-stop`; the list is not virtualized, so DOM order is the order
 * on screen and collapsed content is simply absent.
 */
export type SessionListStopKind = "project-folder" | "group" | "folder" | "session";

export type SessionListWalkDirection = "previous" | "next";

const STOP_SELECTOR = "[data-sidebar-stop]";

/** Every stop inside the rail, in on-screen order. */
export function sessionListStops(root: ParentNode): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(STOP_SELECTOR));
}

export function sessionListStopKind(stop: HTMLElement): SessionListStopKind | null {
  const kind = stop.dataset.sidebarStop;
  return kind === "project-folder" || kind === "group" || kind === "folder" || kind === "session"
    ? kind
    : null;
}

/**
 * Where the walk stands: the stop holding keyboard focus, else the row of the
 * session the focused window shows, else nowhere (-1).
 */
export function currentSessionListStop(
  stops: readonly HTMLElement[],
  activeElement: Element | null,
  sessionId: string | null
): number {
  if (activeElement !== null) {
    const focused = stops.findIndex(stop => stop.contains(activeElement));
    if (focused >= 0) return focused;
  }
  if (sessionId === null) return -1;
  return stops.findIndex(stop => stop.dataset.sessionId === sessionId);
}

/**
 * The stop before or after the current one, wrapping around. With no current
 * stop the walk enters at the first (next) or last (previous) stop; with fewer
 * than two stops there is nowhere to go.
 */
export function adjacentSessionListStop(
  stops: readonly HTMLElement[],
  current: number,
  direction: SessionListWalkDirection
): HTMLElement | null {
  if (stops.length < 2) return null;
  if (current < 0) return (direction === "next" ? stops[0] : stops.at(-1)) ?? null;
  const offset = direction === "next" ? 1 : -1;
  return stops[(current + offset + stops.length) % stops.length] ?? null;
}

/**
 * Lands the walk on a stop. A header, and a session row while a bulk selection
 * is open, takes keyboard focus. Otherwise a session row is activated the way a
 * click activates it; it also takes focus when the walk is
 * already inside the rail, so the next step starts from it, while focus held
 * elsewhere (the composer) stays put and the next step resolves the row from
 * the session the window now shows.
 */
export function landSessionListStop(
  root: Element,
  stop: HTMLElement,
  activeElement: Element | null
) {
  // While a bulk selection is open a click toggles the row's membership, so
  // the walk only focuses the row there instead of changing the selection.
  if (sessionListStopKind(stop) === "session" && !stop.hasAttribute("aria-pressed")) {
    if (activeElement !== null && root.contains(activeElement)) stop.focus();
    stop.click();
  } else {
    stop.focus();
  }
  stop.scrollIntoView({ block: "nearest" });
}

/**
 * Walks one stop through the rail. Returns false when the rail offers nowhere
 * else to go, so the caller can say so.
 */
export function walkSessionList(
  root: Element,
  direction: SessionListWalkDirection,
  sessionId: string | null
): boolean {
  const stops = sessionListStops(root);
  const activeElement = root.ownerDocument.activeElement;
  const current = currentSessionListStop(stops, activeElement, sessionId);
  const target = adjacentSessionListStop(stops, current, direction);
  if (target === null) return false;
  landSessionListStop(root, target, activeElement);
  return true;
}

/**
 * Tree-style disclosure keys for a header toggle: ArrowRight expands and
 * ArrowLeft collapses; each is a no-op (and lets the key through) when the
 * header is already in that state.
 */
export function onDisclosureArrowKey(
  event: KeyboardEvent<HTMLElement>,
  expanded: boolean,
  onToggle: (() => void) | undefined
) {
  if (onToggle === undefined || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) {
    return;
  }
  const wants = event.key === "ArrowRight" ? true : event.key === "ArrowLeft" ? false : undefined;
  if (wants === undefined || wants === expanded) return;
  event.preventDefault();
  onToggle();
}
