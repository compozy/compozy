import type { ComponentProps } from "react";

import { Eyebrow } from "@compozy/ui";

import type { AgentFleetRowModel } from "../lib/agent-fleet-projection";
import { cn } from "@/lib/utils";

interface AgentLayerProvenanceProps extends ComponentProps<"div"> {
  layer: AgentFleetRowModel["layer"];
  shadows: AgentFleetRowModel["shadowLayers"];
}

function AgentLayerProvenance({ layer, shadows, className, ...props }: AgentLayerProvenanceProps) {
  return (
    <div
      className={cn("flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1", className)}
      {...props}
    >
      <span className="inline-flex min-w-0 items-baseline gap-1.5">
        <Eyebrow>Layer</Eyebrow>
        <span className="truncate font-mono text-badge text-muted">{layer}</span>
      </span>
      {shadows.length > 0 ? (
        <span className="inline-flex min-w-0 items-baseline gap-1.5">
          <Eyebrow>Shadows</Eyebrow>
          <span className="truncate font-mono text-badge text-muted">{shadows.join(", ")}</span>
        </span>
      ) : null}
    </div>
  );
}

export { AgentLayerProvenance };
