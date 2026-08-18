import { Layers } from "lucide-react";

import { MonoTag } from "../mono-tag";
import { LoopRailSection } from "../loop-rail-section";

interface LoopVersionsPanelProps {
  version: number;
}

export function LoopVersionsPanel({ version }: LoopVersionsPanelProps) {
  return (
    <LoopRailSection
      data-testid="loop-versions"
      gist={`v${version} current`}
      icon={<Layers aria-hidden="true" className="size-3.5" />}
      title="Versions"
    >
      <div className="px-3.5 py-1">
        <div className="flex items-center gap-2.5 py-2.5">
          <span className="font-mono text-mono-id text-fg-strong">v{version}</span>
          <MonoTag className="rounded-xs bg-success-tint px-1.5 py-0.5 text-success">
            current
          </MonoTag>
        </div>
      </div>
    </LoopRailSection>
  );
}
