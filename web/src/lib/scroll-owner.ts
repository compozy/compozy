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
