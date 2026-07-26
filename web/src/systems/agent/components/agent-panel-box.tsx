import type { ComponentProps } from "react";

import { cn } from "@agh/ui";

/**
 * OpenDesign `panelbox` surface for agent detail lanes — soft canvas, hairline
 * border, clipped corners. Domain composite (not a generic @agh/ui primitive).
 *
 * Frozen reference: docs/design/opendesign/agents/agent-detail.html
 * SHA-1 4a4c214402cc83a06ff8ab7c607b9c0d6cfc12bc
 */
export function AgentPanelBox({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="agent-panel-box"
      className={cn("overflow-hidden rounded-lg border border-line bg-canvas-soft", className)}
      {...props}
    />
  );
}
