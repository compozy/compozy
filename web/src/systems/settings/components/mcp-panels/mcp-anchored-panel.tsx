import type { ReactNode } from "react";

import { Popover, PopoverPanelContent } from "@compozy/ui";

import type { MCPPanelAnchor } from "../../lib/mcp-panel-anchor";

export interface MCPAnchoredPanelProps {
  /** The control the panel opens beside; nothing renders without one. */
  anchor: MCPPanelAnchor | null;
  open: boolean;
  /** Escape, a press outside, or the panel's own Close. */
  onClose: () => void;
  side?: "top" | "bottom" | "left" | "right";
  align?: "start" | "center" | "end";
  children: ReactNode;
}

/**
 * A row panel opened beside the control that started it (non-modal popover). Focus goes back to
 * that control on close while it is still on screen.
 */
export function MCPAnchoredPanel({
  anchor,
  open,
  onClose,
  side = "bottom",
  align = "end",
  children,
}: MCPAnchoredPanelProps) {
  return (
    <Popover
      open={open && anchor !== null}
      modal={false}
      onOpenChange={next => {
        if (!next) onClose();
      }}
    >
      <PopoverPanelContent
        side={side}
        align={align}
        sideOffset={6}
        anchor={anchor?.virtual}
        finalFocus={() => (anchor?.element.isConnected ? anchor.element : false)}
      >
        {children}
      </PopoverPanelContent>
    </Popover>
  );
}
