import { useState } from "react";
import { ChevronDown } from "lucide-react";

import {
  AvatarGroup,
  AvatarGroupCount,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Icon,
  cn,
} from "@compozy/ui";

import { SubagentAvatar } from "./subagent-avatar";
import { SubagentCard, type SubagentOpenOptions } from "./subagent-card";
import { SubagentElapsed } from "./subagent-elapsed";
import {
  subagentGroupClock,
  subagentGroupSummary,
  type SubagentSummaryTone,
} from "./subagent-format";
import type { SubagentView } from "./types";

export interface SubagentGroupProps {
  /** Two or more adjacent subagents from the same parent turn, in delegation order. */
  subagents: readonly SubagentView[];
  onOpen?: (subagent: SubagentView, options: SubagentOpenOptions) => void;
  /** Expansion is window-local per group; the owner may control it. */
  open?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
  stale?: boolean;
  className?: string;
}

const STACK_LIMIT = 3;

const SUMMARY_TONE: Record<SubagentSummaryTone, string> = {
  live: "text-info",
  failed: "text-danger",
  settled: "text-subtle",
};

/**
 * Same-turn delegations folded into one disclosure (transcript VC-03): stacked
 * avatars, `N subagents`, a state summary, the group span and a chevron. A
 * collapsed group whose members all settled dims until hover or focus.
 */
export function SubagentGroup({
  subagents,
  onOpen,
  open,
  defaultOpen = false,
  onOpenChange,
  stale = false,
  className,
}: SubagentGroupProps) {
  const summary = subagentGroupSummary(subagents);
  const stacked = subagents.slice(0, STACK_LIMIT);
  const overflow = subagents.length - stacked.length;
  const [uncontrolledOpen, setUncontrolledOpen] = useState(defaultOpen);
  const expanded = open ?? uncontrolledOpen;
  const dim = summary.settled && !expanded;

  const handleOpenChange = (next: boolean) => {
    if (open === undefined) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };

  return (
    <Collapsible
      open={expanded}
      onOpenChange={handleOpenChange}
      data-slot="subagent-group"
      data-settled={summary.settled ? "true" : undefined}
      data-dim={dim ? "true" : undefined}
      className={cn(
        "group/subagent-group max-w-140 rounded-md border border-line-soft bg-canvas-soft transition-opacity duration-base ease-out motion-reduce:transition-none",
        dim && "opacity-62 focus-within:opacity-100 hover:opacity-100",
        className
      )}
    >
      <CollapsibleTrigger
        render={
          <button
            type="button"
            className="grid min-h-11.5 w-full grid-cols-[auto_minmax(0,1fr)_auto_14px] items-center gap-2.5 rounded-md py-1.75 pr-2.5 pl-2.25 text-left text-fg outline-none transition-colors duration-fast ease-out hover:bg-surface-2 focus-visible:shadow-focus-ring"
          />
        }
      >
        <AvatarGroup className="-space-x-1.75 *:data-[slot=avatar]:ring-canvas-soft">
          {stacked.map(subagent => (
            <SubagentAvatar key={subagent.id} provider={subagent.runtime.provider} />
          ))}
          {overflow > 0 ? (
            <AvatarGroupCount className="w-auto min-w-6 bg-elevated px-1.25 font-mono text-micro font-semibold text-muted tabular-nums ring-canvas-soft">
              +{overflow}
            </AvatarGroupCount>
          ) : null}
        </AvatarGroup>
        <span className="flex min-w-0 flex-col">
          <span className="text-transcript-meta font-medium text-fg-strong">{summary.label}</span>
          <span
            className={cn("truncate text-transcript-caption", SUMMARY_TONE[summary.tone])}
            data-slot="subagent-group-summary"
            data-tone={summary.tone}
          >
            {summary.summary}
          </span>
        </span>
        <SubagentElapsed clock={subagentGroupClock(subagents, { stale })} />
        <Icon
          as={ChevronDown}
          aria-hidden="true"
          className={cn(
            "text-faint transition-transform duration-base ease-out motion-reduce:transition-none",
            expanded && "rotate-180"
          )}
        />
      </CollapsibleTrigger>
      <CollapsibleContent className="flex flex-col gap-0.5 border-t border-line-soft p-1">
        {subagents.map(subagent => (
          <SubagentCard
            key={subagent.id}
            subagent={subagent}
            onOpen={onOpen}
            stale={stale}
            variant="flush"
          />
        ))}
      </CollapsibleContent>
    </Collapsible>
  );
}
