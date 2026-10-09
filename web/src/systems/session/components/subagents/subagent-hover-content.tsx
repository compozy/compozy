import { Zap } from "lucide-react";

import { Icon, KindIcon, PropertyRow, StateGlyph, cn } from "@compozy/ui";

import { SubagentElapsed } from "./subagent-elapsed";
import {
  SUBAGENT_MODEL_NOT_REPORTED,
  SUBAGENT_STATUS_GLYPH,
  SUBAGENT_STATUS_WORD,
  subagentElapsedClock,
  subagentHoverPreview,
  subagentLocationRows,
  subagentRuntimeLabel,
} from "./subagent-format";
import type { SubagentLocationView, SubagentView } from "./types";

export interface SubagentHoverContentProps {
  subagent: SubagentView;
  /** Child workspace/worktree; a row shows only where it differs from `parentLocation`. */
  location?: SubagentLocationView;
  parentLocation?: SubagentLocationView;
  stale?: boolean;
}

/**
 * The metadata the card row leaves out (transcript VC-04): full title, runtime,
 * status with elapsed, location when it differs from the parent, and the
 * progress or result preview.
 */
export function SubagentHoverContent({
  subagent,
  location,
  parentLocation,
  stale = false,
}: SubagentHoverContentProps) {
  const runtime = subagentRuntimeLabel(subagent);
  const failed = subagent.status === "failed";
  const preview = subagentHoverPreview(subagent);
  const locationRows = subagentLocationRows(location, parentLocation);

  return (
    <div className="flex flex-col gap-2" data-slot="subagent-hover" data-status={subagent.status}>
      <p className="text-transcript-body font-semibold text-pretty text-fg-strong">
        {subagent.title}
      </p>
      <div className="flex flex-col gap-1.25 text-transcript-caption text-muted">
        <div className="flex min-w-0 items-center gap-1.5" data-slot="subagent-hover-runtime">
          <KindIcon kind={subagent.runtime.provider ?? undefined} size="xs" tone="muted" />
          {runtime.model === null ? (
            <span className="text-subtle italic">{SUBAGENT_MODEL_NOT_REPORTED}</span>
          ) : (
            <span className="min-w-0 truncate">
              <span className="font-medium text-fg">{runtime.model}</span>
              {runtime.effort ? (
                <>
                  <span className="text-faint"> · </span>
                  {runtime.effort}
                </>
              ) : null}
            </span>
          )}
          {runtime.fast ? (
            <Icon as={Zap} size="sm" aria-label="Fast" role="img" className="text-subtle" />
          ) : null}
        </div>
        <div
          className={cn("flex min-w-0 items-center gap-1.5", failed && "text-danger")}
          data-slot="subagent-hover-status"
        >
          <StateGlyph size="sm" state={SUBAGENT_STATUS_GLYPH[subagent.status]} still={stale} />
          <span>{SUBAGENT_STATUS_WORD[subagent.status]}</span>
          <SubagentElapsed className="ml-auto" clock={subagentElapsedClock(subagent, { stale })} />
        </div>
        {locationRows.map(row => (
          <PropertyRow
            key={row.label}
            label={row.label}
            mono
            className="min-h-0 py-0"
            valueTitle={row.value}
          >
            {row.value}
          </PropertyRow>
        ))}
      </div>
      {preview ? (
        <p
          className={cn(
            "border-t border-line-soft pt-2 text-transcript-caption text-pretty wrap-anywhere text-fg-2",
            failed && "text-danger"
          )}
          data-slot="subagent-hover-preview"
        >
          {preview}
        </p>
      ) : null}
    </div>
  );
}
