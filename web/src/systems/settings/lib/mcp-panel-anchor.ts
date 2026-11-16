/**
 * Where a row panel opens: beside the control that started it. The control can unmount while the
 * panel is still open (a row that turns Ready drops its Sign in button, a composer row closes), so
 * the panel keeps the control's last position instead of jumping to the corner.
 */
export interface MCPPanelAnchor {
  element: HTMLElement;
  virtual: { getBoundingClientRect: () => DOMRect; contextElement: Element };
}

export function mcpPanelAnchor(element: HTMLElement): MCPPanelAnchor {
  let last = element.getBoundingClientRect();
  return {
    element,
    virtual: {
      contextElement: element,
      getBoundingClientRect: () => {
        if (element.isConnected) last = element.getBoundingClientRect();
        return last;
      },
    },
  };
}
