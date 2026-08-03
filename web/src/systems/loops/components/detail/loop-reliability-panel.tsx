import { Hand, Hourglass, RotateCcw, Timer } from "lucide-react";

import { Section } from "@compozy/ui";

import { buildLoopReliability, type LoopReliabilityIcon } from "../../lib/loop-reliability";
import type { LoopGraph } from "../../lib/loop-graph";
import type { LoopDetail } from "../../types";

const TILE_ICON: Record<LoopReliabilityIcon, typeof RotateCcw> = {
  retry: RotateCcw,
  deadline: Timer,
  wait: Hourglass,
  control: Hand,
};

interface LoopReliabilityPanelProps {
  effectiveLifecycle: LoopDetail["effective_lifecycle"];
  graph: LoopGraph;
}

/**
 * "When something goes wrong" — the loop's failure posture in plain language.
 *
 * Each tile restates a Spec 1 guarantee the daemon reports for this loop; a guarantee
 * the definition does not declare gets no tile at all. Neutral by design: none of this
 * is live state, so nothing here carries signal color.
 */
export function LoopReliabilityPanel({ effectiveLifecycle, graph }: LoopReliabilityPanelProps) {
  const tiles = buildLoopReliability({ effectiveLifecycle, graph });
  if (tiles.length === 0) return null;
  return (
    <Section label="When something goes wrong">
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2" data-testid="loop-reliability">
        {tiles.map(tile => {
          const Icon = TILE_ICON[tile.icon];
          return (
            <article
              className="flex flex-col gap-2 rounded-lg border border-line bg-canvas-soft px-4 py-3.5"
              data-testid={`loop-reliability-${tile.id}`}
              key={tile.id}
            >
              <div className="flex items-center gap-2">
                <Icon aria-hidden="true" className="size-3.5 text-muted" />
                <h3 className="text-form-label font-medium text-fg-strong">{tile.title}</h3>
              </div>
              <p className="text-form-hint leading-relaxed text-subtle">{tile.body}</p>
              <p className="font-mono text-mono-id text-faint">{tile.codes.join(" · ")}</p>
            </article>
          );
        })}
      </div>
    </Section>
  );
}
