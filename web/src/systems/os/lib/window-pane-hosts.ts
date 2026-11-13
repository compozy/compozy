import {
  isRenderedElement as isRendered,
  SCROLL_OWNER_ATTRIBUTE,
  SCROLL_RESTORE_EVENT,
} from "@/lib/scroll-owner";

import type { OsWindowRoute } from "./os-types";
import { PaneScrollSettler } from "./pane-scroll-settler";
import { sameOsWindowRoute } from "./window-manager-route";

const HOST_SLOT = "os-window-host";
/** Below this many recorded containers a pane is never swept. */
const SWEEP_FLOOR = 16;
/** Where re-insertions stop mattering: the desk layer holds every desktop, tab and slot. */
const DESK_LAYER = '[data-slot="os-desk-layer"]';

interface ScrollOffset {
  top: number;
  left: number;
  /** The window's route when the container scrolled: the content the offset belongs to. */
  route: OsWindowRoute | null;
}

interface PaneHost {
  windowId: string;
  element: HTMLElement;
  /** The last offsets each scroll container inside the pane reported. */
  offsets: Map<Element, ScrollOffset>;
  /** Recording a new container past this size sweeps the ones no longer in the pane. */
  sweepAt: number;
  /** Whether the pane's surface had a box at the last observation; `null` before the first. */
  rendered: boolean | null;
  /** The pane roots the resize observer watches, so they can be released by name. */
  observed: Set<Element>;
  /** The window's route when the hosts last read it (`syncRoutes`). */
  route: OsWindowRoute | null;
}

/** What the hosts read from the window manager, at the moment they act. */
export interface PaneHostWindows {
  /** Whether the window holds this client's authoritative focus now. */
  isFocused(windowId: string): boolean;
  /** The window's current route. */
  routeOf(windowId: string): OsWindowRoute | null;
}

const OWNER_SELECTOR = `[${SCROLL_OWNER_ATTRIBUTE}]`;
/** Input that is the operator taking a pane's scroll, ending any restore still settling there. */
const GESTURES = ["wheel", "pointerdown", "keydown", "touchstart"] as const;

/**
 * One DOM host per window, owned by a desktop's tab layer. A window's pane renders into its host
 * through a portal whose container never changes, and the split slot the window occupies adopts
 * the host node. Moving a window between tabs or positions in a split then moves a DOM node; the
 * pane, its app state and its data stay mounted instead of remounting under a new React parent.
 *
 * A subtree taken out of the document, or not rendered (a tab not showing, a member behind a
 * collapsed split), loses the scroll offsets of its scroll containers and its focused element.
 * The host keeps both for every app:
 * - A capture-phase `scroll` listener on the host records each container's offsets as they change
 *   (one map write per scroll event; the offsets are already laid out when it fires), with the
 *   window's route at that moment. Containers that left the pane (an in-window navigation
 *   replaced them) are swept when a new container is recorded past twice the size of the last
 *   sweep, and when the pane's root changes: amortized O(1) per scroll, and the map stays within
 *   twice the containers the pane still holds.
 * - Placing the host in a slot, re-inserting it with an ancestor (a keyed reorder of tab layers,
 *   collapsed split members or desktops moves their DOM nodes without remounting a slot), and the
 *   pane's surface getting a box again after it had none write back every recorded offset the
 *   browser dropped (now 0), which lays the pane out once. An offset belongs to the content that
 *   scrolled: one recorded under another route (a `window navigate` while the pane was hidden, a
 *   route change just before it hid) is dropped instead of written onto the new content. A
 *   container that stays in the pane across a route change while the pane is rendered (a section
 *   rail, a list beside its detail, a search-only change) kept its content and its real offset:
 *   `syncRoutes` moves its recorded offset to the new route, so it is restored as usual. Only a
 *   rendered container records: a hidden one reads 0, which is no evidence of its position. A
 *   re-insertion is seen by a `childList` mutation observer on the slot's ancestors up to the
 *   desk layer: those lists change only when the layout does, never with an app's content. The
 *   same observer watches each host's own children, so a pane root that mounts after `place` is
 *   observed for size too.
 *   It runs before the apps' own resize handling for that frame (its observer is created first).
 *   Content that cannot hold its offset at that moment (a virtualized list whose rows unmounted
 *   while hidden, a code view that measured its lines at 0) gets it when it grows, and an offset
 *   the container's own logic drops back to 0 is written again, for a bounded settling time
 *   (`PaneScrollSettler`); an app that moves the container anywhere else keeps its move.
 * - Focus that last landed inside the pane returns to that element (`preventScroll`) when it was
 *   lost to the move or the hiding: only while nothing else holds focus (`body`), so a drag
 *   source or a tab the operator clicked keeps it, and only while the pane's window holds the
 *   client's authoritative focus. A passive relocation never changes routing intent: the pane's
 *   focus capture reads a focus landing in an unfocused window as the operator choosing it, so a
 *   restore there would take focus from the window an open, a peer's move or a directional focus
 *   gave it. In the focused window the capture is a no-op (`useOsWindowMember` sends nothing for
 *   the window that already has focus). The remembered element is kept while its window is not
 *   focused: the next restore after the window has focus again (a show, a move) still returns
 *   the caret where the operator left it.
 * A container that declares itself a scroll owner (`data-scroll-owner`, the transcript) keeps its
 * own position: the host records nothing for it and, instead of writing its offset, dispatches
 * `SCROLL_RESTORE_EVENT` to it at the same moments, so one writer moves it.
 */
export class WindowPaneHosts {
  private readonly windows: PaneHostWindows;
  private readonly hosts = new Map<string, PaneHost>();
  private readonly byElement = new WeakMap<Element, PaneHost>();
  private readonly settler = new PaneScrollSettler();
  /** Where focus last landed inside a pane, while nothing outside the panes took it since. */
  private lastFocus: { windowId: string; element: HTMLElement } | null = null;
  private resizeObserver: ResizeObserver | null = null;
  private mutationObserver: MutationObserver | null = null;
  private listening: Document | null = null;
  /** Set while a host leaves its slot: the blur that causes is not the operator's. */
  private detaching = false;

  constructor(windows: PaneHostWindows) {
    this.windows = windows;
  }

  /** The host for a window, created on first use; a `display: contents` box adds no layout. */
  hostOf(windowId: string): HTMLElement {
    return this.ensure(windowId).element;
  }

  /**
   * Puts the window's host in `slot` and restores what leaving its previous slot lost. Returns
   * the removal, for the slot's cleanup.
   */
  place(windowId: string, slot: HTMLElement): () => void {
    const host = this.ensure(windowId);
    slot.appendChild(host.element);
    this.listen(slot.ownerDocument);
    // The mutations queued so far include this slot's own mount, which would read as a
    // re-insertion of the host just restored: handle them now, without restoring it twice. A
    // re-insertion after this call is a new move and reaches the observer as usual.
    const pending = this.mutationObserver?.takeRecords() ?? [];
    this.mutationObserver?.observe(host.element, { childList: true });
    this.trackRoots(host);
    this.watchAncestors(slot);
    this.restore(host);
    this.handleMutations(pending, host);
    return () => {
      if (host.element.parentNode !== slot) return;
      const active = slot.ownerDocument.activeElement;
      this.detaching = true;
      try {
        slot.removeChild(host.element);
      } finally {
        this.detaching = false;
      }
      // Focus the removal took from the pane comes back once it is placed again.
      if (active instanceof HTMLElement && host.element.contains(active)) {
        this.lastFocus = { windowId, element: active };
      }
    };
  }

  /**
   * Reads every window's route. A route change ends the restores still settling in its pane (an
   * in-window navigation that reset a container is the app's move, not a dropped offset) and,
   * while the pane is rendered, keeps the offsets of the containers that outlived it.
   */
  syncRoutes(): void {
    for (const host of this.hosts.values()) {
      const route = this.windows.routeOf(host.windowId);
      if (sameOsWindowRoute(host.route, route)) continue;
      host.route = route;
      this.settler.releaseOwner(host);
      if (host.rendered === false || !isRendered(host.element)) continue;
      for (const [container, offset] of host.offsets) {
        // A hidden container kept no evidence of the new content: its offset stays dropped.
        if (host.element.contains(container) && isRendered(container)) offset.route = route;
      }
    }
  }

  /** Drops the hosts of windows the layer no longer renders. */
  retain(windowIds: ReadonlySet<string>): void {
    for (const [windowId, host] of this.hosts) {
      if (windowIds.has(windowId)) continue;
      // By the roots it observed: the portal's children are already gone by now.
      for (const root of host.observed) this.resizeObserver?.unobserve(root);
      host.observed.clear();
      this.hosts.delete(windowId);
      if (this.lastFocus?.windowId === windowId) this.lastFocus = null;
      this.settler.releaseOwner(host);
    }
  }

  /** Stops listening; the next `place` listens again. */
  dispose(): void {
    this.resizeObserver?.disconnect();
    this.resizeObserver = null;
    this.mutationObserver?.disconnect();
    this.mutationObserver = null;
    this.listening?.removeEventListener("focusin", this.handleFocusIn, true);
    this.listening?.removeEventListener("focusout", this.handleFocusOut, true);
    this.listening = null;
    this.settler.dispose();
    for (const host of this.hosts.values()) {
      host.rendered = null;
      host.observed.clear();
    }
  }

  private ensure(windowId: string): PaneHost {
    const existing = this.hosts.get(windowId);
    if (existing !== undefined) return existing;
    const element = document.createElement("div");
    element.style.display = "contents";
    element.dataset.slot = HOST_SLOT;
    const host: PaneHost = {
      windowId,
      element,
      offsets: new Map(),
      sweepAt: SWEEP_FLOOR,
      rendered: null,
      observed: new Set(),
      route: this.windows.routeOf(windowId),
    };
    element.addEventListener(
      "scroll",
      event => {
        const target = event.target;
        if (target instanceof Element) this.record(host, target);
      },
      { capture: true, passive: true }
    );
    for (const gesture of GESTURES) {
      element.addEventListener(gesture, () => this.settler.releaseOwner(host), {
        capture: true,
        passive: true,
      });
    }
    this.hosts.set(windowId, host);
    this.byElement.set(element, host);
    return host;
  }

  private listen(document: Document): void {
    if (this.listening === document) return;
    this.listening = document;
    document.addEventListener("focusin", this.handleFocusIn, true);
    document.addEventListener("focusout", this.handleFocusOut, true);
    if (typeof MutationObserver !== "undefined") {
      this.mutationObserver = new MutationObserver(records => this.handleMutations(records));
    }
    if (typeof ResizeObserver === "undefined") return;
    this.resizeObserver = new ResizeObserver(entries => {
      for (const entry of entries) this.observeBox(entry);
    });
  }

  /** Observes the host's current pane roots for size and releases the ones that left it. */
  private trackRoots(host: PaneHost): void {
    for (const root of host.observed) {
      if (root.parentElement === host.element) continue;
      this.resizeObserver?.unobserve(root);
      host.observed.delete(root);
    }
    for (const root of host.element.children) {
      if (host.observed.has(root)) continue;
      host.observed.add(root);
      this.resizeObserver?.observe(root);
    }
  }

  /** Watches the lists a keyed reorder can re-insert the slot through, up to the desk layer. */
  private watchAncestors(slot: HTMLElement): void {
    const body = slot.ownerDocument.body;
    for (let node = slot.parentElement; node !== null && node !== body; node = node.parentElement) {
      this.mutationObserver?.observe(node, { childList: true });
      if (node.matches(DESK_LAYER)) return;
    }
  }

  /** `placed`: the host `place` just restored, which these records do not move again. */
  private handleMutations(records: readonly MutationRecord[], placed?: PaneHost): void {
    const reinserted = new Set<PaneHost>();
    for (const record of records) {
      const own = record.target instanceof Element ? this.byElement.get(record.target) : undefined;
      if (own !== undefined) {
        this.trackRoots(own);
        this.sweep(own);
        continue;
      }
      for (const node of record.addedNodes) {
        if (!(node instanceof Element)) continue;
        for (const host of this.hosts.values()) {
          if (host !== placed && node.contains(host.element)) reinserted.add(host);
        }
      }
    }
    for (const host of reinserted) this.restore(host);
  }

  /** A surface with no box was hidden; one that gets a box back is shown again. */
  private observeBox(entry: ResizeObserverEntry): void {
    const parent = entry.target.parentElement;
    const host = parent === null ? undefined : this.byElement.get(parent);
    if (host === undefined) return;
    const rendered = entry.contentRect.width > 0 || entry.contentRect.height > 0;
    const shownAgain = rendered && host.rendered === false;
    host.rendered = rendered;
    if (!rendered) this.settler.releaseOwner(host);
    if (shownAgain) this.restore(host);
  }

  private record(host: PaneHost, container: Element): void {
    // An owner keeps its own position; a restore still settling consumes the scrolls it causes.
    if (container.hasAttribute(SCROLL_OWNER_ATTRIBUTE) || this.settler.scrolled(container)) return;
    // A container that is not rendered (its pane hidden or detached mid-move) reads 0.
    if (!isRendered(container)) return;
    const known = host.offsets.has(container);
    host.offsets.set(container, {
      top: container.scrollTop,
      left: container.scrollLeft,
      route: this.windows.routeOf(host.windowId),
    });
    if (!known && host.offsets.size > host.sweepAt) this.sweep(host);
  }

  /** Drops the containers no longer in the pane; the next sweep waits for twice what is left. */
  private sweep(host: PaneHost): void {
    for (const container of host.offsets.keys()) {
      if (!host.element.contains(container)) host.offsets.delete(container);
    }
    host.sweepAt = Math.max(SWEEP_FLOOR, host.offsets.size * 2);
  }

  private restore(host: PaneHost): void {
    // A restore while the pane renders is its show: the box the resize observer reports for it
    // next is not a second one.
    if (isRendered(host.element)) host.rendered = true;
    const route = this.windows.routeOf(host.windowId);
    for (const [element, offset] of host.offsets) {
      if (!host.element.contains(element) || !sameOsWindowRoute(offset.route, route)) {
        host.offsets.delete(element);
        continue;
      }
      if (!isRendered(element)) continue;
      // Only an offset the browser dropped: one the app already set again (a terminal syncing
      // to its buffer) is the app's. Content that cannot hold it yet gets it once it grows.
      this.settler.restore(element, offset, host);
    }
    // Scroll owners restore themselves, in the same frame.
    for (const owner of host.element.querySelectorAll(OWNER_SELECTOR)) {
      if (isRendered(owner)) owner.dispatchEvent(new Event(SCROLL_RESTORE_EVENT));
    }
    const focus = this.lastFocus;
    if (focus === null || focus.windowId !== host.windowId) return;
    const active = host.element.ownerDocument.activeElement;
    const unclaimed = active === null || active === host.element.ownerDocument.body;
    if (!unclaimed || !host.element.contains(focus.element) || !isRendered(focus.element)) return;
    if (!this.windows.isFocused(host.windowId)) return;
    focus.element.focus({ preventScroll: true });
  }

  private readonly handleFocusIn = (event: FocusEvent): void => {
    const target = event.target;
    if (!(target instanceof HTMLElement)) return;
    const hostElement = target.closest(`[data-slot="${HOST_SLOT}"]`);
    const host = hostElement === null ? undefined : this.byElement.get(hostElement);
    this.lastFocus = host === undefined ? null : { windowId: host.windowId, element: target };
  };

  /** Focus the operator moved to nothing (a click on an empty area) is not given back later. */
  private readonly handleFocusOut = (event: FocusEvent): void => {
    if (this.detaching || event.relatedTarget !== null) return;
    if (this.lastFocus?.element !== event.target) return;
    if (isRendered(this.lastFocus.element)) this.lastFocus = null;
  };
}
