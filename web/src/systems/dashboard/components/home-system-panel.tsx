import { ChevronDown } from "lucide-react";

import {
  CodeBlock,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  MetadataTile,
  Panel,
  Pill,
} from "@agh/ui";

import type { HomeSystemModel } from "../hooks/use-home-system";

export interface HomeSystemPanelProps {
  system: HomeSystemModel;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Zone 7 — the operator depth, folded by default. One truthful summary line;
 * the expanded tiles and the CLI echo carry the full system state.
 */
export function HomeSystemPanel({ system, open, onOpenChange }: HomeSystemPanelProps) {
  if (system.tiles.length === 0) {
    return null;
  }

  return (
    <Panel bodyClassName="p-0" data-slot="home-system">
      <Collapsible onOpenChange={onOpenChange} open={open}>
        <CollapsibleTrigger className="group flex w-full items-center gap-2.5 px-4 py-3 text-left transition-colors duration-base hover:bg-row-hover focus-visible:shadow-focus-inset focus-visible:outline-none">
          <Pill.Dot tone={system.allNormal ? "success" : "warning"} />
          <span className="shrink-0 text-small-body font-medium text-fg-strong">
            {system.allNormal ? "All systems normal" : "Needs a look"}
          </span>
          <span className="min-w-0 flex-1 truncate font-mono text-mono-id tabular-nums text-subtle">
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
            <CodeBlock
              className="mt-3"
              code={"# same view from the CLI\nagh observe overview -o json"}
              language="bash"
            />
          </div>
        </CollapsibleContent>
      </Collapsible>
    </Panel>
  );
}
