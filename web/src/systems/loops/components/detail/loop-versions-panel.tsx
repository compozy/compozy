import { Section } from "@agh/ui";

import { MonoTag } from "../mono-tag";

interface LoopVersionsPanelProps {
  version: number;
}

/**
 * Right-rail Versions panel. The Loop resource projection exposes only the current
 * published version integer (no server-side draft store or version history in v1,
 * §9.13), so exactly the current version renders here — truthful UI, no invented
 * history rows. Version diff/revert are deferred.
 */
export function LoopVersionsPanel({ version }: LoopVersionsPanelProps) {
  return (
    <Section label="Versions" data-testid="loop-versions">
      <div className="rounded-lg border border-line bg-canvas-soft px-3.5 py-1">
        <div className="flex items-center gap-2.5 py-2.5">
          <span className="font-mono text-mono-id text-fg-strong">v{version}</span>
          <MonoTag className="rounded-xs bg-success-tint px-1.5 py-0.5 text-success">
            current
          </MonoTag>
        </div>
      </div>
    </Section>
  );
}
