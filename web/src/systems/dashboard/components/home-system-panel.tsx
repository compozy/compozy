import { ChevronDown } from "lucide-react";

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  MetadataTile,
  Panel,
  Pill,
  SkeletonRegion,
} from "@compozy/ui";

import type { HomeSystemModel } from "../lib/home-system";
import { HOME_SYSTEM_ROW_CLASS } from "./home-zone-shape";
import { HomeSystemPanelSkeleton } from "./home-zone-skeletons";

export interface HomeSystemPanelProps {
  system: HomeSystemModel;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Daemon status has not answered yet: the row waits instead of reporting "Needs a look". */
  loading?: boolean;
}

/**
 * Zone 7 — the operator depth, folded by default. One truthful summary line;
 * the expanded tiles carry the full system state.
 */
export function HomeSystemPanel({
  system,
  open,
  onOpenChange,
  loading = false,
}: HomeSystemPanelProps) {
  if (loading) {
    return (
      <SkeletonRegion label="Loading system status">
        <HomeSystemPanelSkeleton />
      </SkeletonRegion>
    );
  }
  if (system.tiles.length === 0) {
    return null;
  }

  return (
    <Panel bodyClassName="p-0" data-slot="home-system">
      <Collapsible onOpenChange={onOpenChange} open={open}>
        <CollapsibleTrigger
          className={`group ${HOME_SYSTEM_ROW_CLASS} text-left transition-colors duration-base hover:bg-surface-2 focus-visible:shadow-focus-inset focus-visible:outline-none`}
        >
          <Pill.Dot tone={system.allNormal ? "success" : "warning"} />
          <span className="shrink-0 text-small-body font-medium text-fg-strong">
            {system.allNormal ? "All systems normal" : "Needs a look"}
          </span>
          <span className="min-w-0 flex-1 truncate text-small-body text-muted">
            {system.summary}
          </span>
          <ChevronDown
            aria-hidden="true"
            className="size-3.5 shrink-0 text-faint transition-transform duration-base group-data-[panel-open]:rotate-180"
          />
        </CollapsibleTrigger>
        <CollapsibleContent>
          <div className="border-t border-line-soft p-4">
            <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2 min-[1080px]:grid-cols-3">
              {system.tiles.map(tile => (
                <MetadataTile
                  detail={
                    <span className="inline-flex items-center gap-1.5">
                      {tile.tone ? <Pill.Dot size="sm" tone={tile.tone} /> : null}
                      {tile.detail}
                    </span>
                  }
                  key={tile.key}
                  label={tile.label}
                  value={tile.value}
                />
              ))}
            </div>
          </div>
        </CollapsibleContent>
      </Collapsible>
    </Panel>
  );
}
