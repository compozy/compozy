import { ArrowUpRight } from "lucide-react";

import { Button, Eyebrow, Item, ItemActions, ItemContent, ItemFooter, ItemHeader } from "@agh/ui";

import { cn } from "@/lib/utils";

import type { OpenWorkEntry } from "../../hooks/use-work";
import { formatNetworkRelativeTime } from "../../lib/network-formatters";
import { WorkChip } from "./work-chip";

export interface WorkInspectorRowProps {
  entry: OpenWorkEntry;
  onJump?: (entry: OpenWorkEntry) => void;
  className?: string;
}

export function WorkInspectorRow({ entry, onJump, className }: WorkInspectorRowProps) {
  const opened = formatNetworkRelativeTime(entry.openedAt);
  const target = entry.targetPeerId ?? "unassigned";

  return (
    <Item
      className={cn("rounded-none px-4 py-3", className)}
      data-testid={`network-work-inspector-row-${entry.workId}`}
    >
      <ItemContent>
        <ItemHeader>
          <WorkChip startedAt={entry.openedAt} state={entry.state} />
          <ItemActions>
            <Button
              aria-label="Jump to message"
              data-testid={`network-work-inspector-jump-${entry.workId}`}
              onClick={onJump ? () => onJump(entry) : undefined}
              size="icon-sm"
              type="button"
              variant="ghost"
            >
              <ArrowUpRight aria-hidden="true" className="size-3" />
            </Button>
          </ItemActions>
        </ItemHeader>
        <p className="font-mono text-eyebrow text-muted">
          <span className="text-subtle">target </span>
          {target}
        </p>
        <ItemFooter>
          <Eyebrow>opened {opened}</Eyebrow>
        </ItemFooter>
      </ItemContent>
    </Item>
  );
}
