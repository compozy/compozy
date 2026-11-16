/**
 * Where a row panel opens: beside the control that started it. The control can unmount while the
 * panel is still open (a row that turns Ready drops its Sign in button, a composer row closes), so
 * the panel keeps the control's last position instead of jumping to the corner.
 */
export interface MCPPanelAnchor {
  element: HTMLElement;
  /** The row holding the control: pressing it is not a press outside the panel. */
  row: HTMLElement;
  /**
   * Where focus returns when the control is gone on close: the popover list the control sat in,
   * when there is one.
   */
  fallback: HTMLElement | null;
  virtual: { getBoundingClientRect: () => DOMRect; contextElement: Element };
}

const ROW_SELECTOR = "li, tr, [role='row']";
const LIST_SELECTOR = "[data-slot='popover-content']";

export function mcpPanelAnchor(element: HTMLElement): MCPPanelAnchor {
  let last = element.getBoundingClientRect();
  return {
    element,
    row: element.closest<HTMLElement>(ROW_SELECTOR) ?? element,
    fallback: element.closest<HTMLElement>(LIST_SELECTOR),
    virtual: {
      contextElement: element,
      getBoundingClientRect: () => {
        if (element.isConnected) last = element.getBoundingClientRect();
        return last;
      },
    },
  };
}

/** The element focus goes back to when a row panel closes; `false` when none is on screen. */
export function mcpPanelReturnFocus(anchor: MCPPanelAnchor | null): HTMLElement | false {
  if (!anchor) return false;
  if (anchor.element.isConnected) return anchor.element;
  if (anchor.fallback?.isConnected) return anchor.fallback;
  return false;
}

/** A press on the row that opened the panel toggles it from the row, not as a press outside. */
export function mcpPanelOwnsPress(anchor: MCPPanelAnchor | null, target: EventTarget | null) {
  return anchor !== null && target instanceof Node && anchor.row.contains(target);
}
