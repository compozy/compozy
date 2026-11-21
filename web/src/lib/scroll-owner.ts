/**
 * A scroll container that keeps its own position across being hidden and shown again declares
 * itself with this attribute. A host that moves or hides panes leaves such a container's offsets
 * alone and, when the container is rendered again, tells it with {@link SCROLL_RESTORE_EVENT}
 * instead of writing `scrollTop` itself: one writer per container.
 */
export const SCROLL_OWNER_ATTRIBUTE = "data-scroll-owner";

/**
 * Dispatched (not bubbling) on a rendered scroll owner when its pane is placed, re-inserted or
 * shown again, before the frame paints. The owner restores its position in the handler.
 */
export const SCROLL_RESTORE_EVENT = "compozy:scroll-restore";

/**
 * Whether the browser renders the element: connected and not inside a `hidden` subtree (a pane,
 * its tab or a collapsed split member). A scroller that is not rendered reads every box and
 * offset as 0, which is no evidence of where its content is.
 */
export function isRenderedElement(element: Element): boolean {
  return element.isConnected && element.closest("[hidden]") === null;
}

interface ScrollRenderTracker {
  /** Whether the element was rendered when its latest scroll event was dispatched. */
  rendered: boolean;
  /** The listeners that read {@link isRenderedAtScroll} for this element. */
  readers: number;
  readonly handleScroll: () => void;
}

const scrollRenderTrackers = new WeakMap<Element, ScrollRenderTracker>();

/**
 * Walks the element's ancestors once per scroll event for every listener that reads
 * {@link isRenderedAtScroll}. The walk runs in the capture phase, which on the target precedes
 * every listener registered without `capture`, so the result is current for each of them whatever
 * order they registered in. Returns the release; the last release removes the walk.
 */
export function trackRenderedOnScroll(element: Element): () => void {
  let tracker = scrollRenderTrackers.get(element);
  if (tracker === undefined) {
    const created: ScrollRenderTracker = {
      rendered: isRenderedElement(element),
      readers: 0,
      handleScroll: () => {
        created.rendered = isRenderedElement(element);
      },
    };
    element.addEventListener("scroll", created.handleScroll, { capture: true, passive: true });
    scrollRenderTrackers.set(element, created);
    tracker = created;
  }
  const tracked = tracker;
  tracked.readers += 1;
  let released = false;
  return () => {
    if (released) return;
    released = true;
    tracked.readers -= 1;
    if (tracked.readers > 0) return;
    element.removeEventListener("scroll", tracked.handleScroll, { capture: true });
    scrollRenderTrackers.delete(element);
  };
}

/**
 * {@link isRenderedElement} for a scroll listener on an element {@link trackRenderedOnScroll}
 * tracks: the answer the one walk for the scroll event being dispatched gave. Only valid while that
 * event is dispatched; an untracked element walks its ancestors.
 */
export function isRenderedAtScroll(element: Element): boolean {
  return scrollRenderTrackers.get(element)?.rendered ?? isRenderedElement(element);
}
