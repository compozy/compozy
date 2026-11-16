import type { ReactNode } from "react";

import { Popover, PopoverPanelContent } from "@compozy/ui";

import {
  mcpPanelOwnsPress,
  mcpPanelReturnFocus,
  type MCPPanelAnchor,
} from "../../lib/mcp-panel-anchor";

export interface MCPAnchoredPanelProps {
  /** The control the panel opens beside; nothing renders without one. */
  anchor: MCPPanelAnchor | null;
  open: boolean;
  /** Escape, a press outside the panel and its row, or the panel's own Close. */
  onClose: () => void;
  side?: "top" | "bottom" | "left" | "right";
  align?: "start" | "center" | "end";
  children: ReactNode;
}

/**
 * A row panel opened beside the control that started it (non-modal popover). Only Escape, a press
 * outside both the panel and the row that opened it, or the panel's own buttons close it: focus
 * leaving it (a sign-in opening the browser) keeps it open. Focus goes back to the control on
 * close, or to the list it sat in when the control is gone.
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
      onOpenChange={(next, details) => {
        if (next) return;
        const dismissed =
          details.reason === "escape-key" ||
          details.reason === "close-press" ||
          (details.reason === "outside-press" && !mcpPanelOwnsPress(anchor, details.event.target));
        // Anything else (focus leaving for the browser, a press on its own row) is refused, so
        // the popover stays open in its own state as well as in ours.
        if (!dismissed) {
          details.cancel();
          return;
        }
        onClose();
      }}
    >
      <PopoverPanelContent
        side={side}
        align={align}
        sideOffset={6}
        anchor={anchor?.virtual}
        finalFocus={() => mcpPanelReturnFocus(anchor)}
      >
        {children}
      </PopoverPanelContent>
    </Popover>
  );
}
