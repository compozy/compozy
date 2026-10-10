import { Zap } from "lucide-react";

import { Icon, KindIcon, PropertyRow, StateGlyph, Time, cn } from "@compozy/ui";

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
import { SubagentPullRequestLink } from "./subagent-row-trail";
import {
  subagentWorktree,
  subagentWorktreeFacts,
  type SubagentWorktreeFact,
} from "./subagent-worktree-format";
import type { SubagentLocationView, SubagentView } from "./types";

type HoverFact = SubagentWorktreeFact | { label: "Workspace"; kind: "mono"; value: string };

/** A composed fact value; plain strings go straight to `PropertyRow` (its truncation and title). */
function HoverFactValue({ fact }: { fact: Exclude<HoverFact, { kind: "mono" }> }) {
  switch (fact.kind) {
    case "base":
      return (
        <>
          <span className="min-w-0 truncate">{fact.ref}</span>
          {fact.sha ? <span className="shrink-0 text-subtle">· {fact.sha}</span> : null}
        </>
      );
    case "commits":
      return (
        <>
          {fact.ahead ? <span>{fact.ahead}</span> : null}
          {fact.ahead && fact.changes ? <span className="text-subtle">·</span> : null}
          {fact.changes ? (
            <span className={cn(fact.changes.dirty && "text-warning")}>{fact.changes.text}</span>
          ) : null}
        </>
      );
    case "pull-request":
      return (
        <>
          <SubagentPullRequestLink pr={fact.pr} size="row" className="-ml-1" />
          <span>{fact.pr.state}</span>
        </>
      );
    case "pull-request-note":
      return <span className="text-subtle">{fact.text}</span>;
    case "observed":
      return <Time iso={fact.iso} className="text-subtle" />;
  }
}

function hoverFactTitle(fact: HoverFact): string | undefined {
  if (fact.kind === "base") return fact.sha ? `${fact.ref} · ${fact.sha}` : fact.ref;
  return undefined;
}

export interface SubagentHoverContentProps {
  subagent: SubagentView;
  /** Child workspace/worktree; a row shows only where it differs from `parentLocation`. */
  location?: SubagentLocationView;
  parentLocation?: SubagentLocationView;
  stale?: boolean;
}

/**
 * The metadata the card row leaves out (transcript VC-04): full title, runtime,
 * status with elapsed, location when it differs from the parent, an isolated
 * subagent's worktree facts (S5, VC-07), and the progress or result preview.
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
  const worktree = subagentWorktree(subagent);
  // The isolated worktree's own name replaces a generic location Worktree row.
  const facts: HoverFact[] = [
    ...subagentLocationRows(location, parentLocation).flatMap(row =>
      worktree !== null && row.label === "Worktree"
        ? []
        : [{ label: row.label, kind: "mono" as const, value: row.value }]
    ),
    ...(worktree ? subagentWorktreeFacts(worktree) : []),
  ];

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
      </div>
      {facts.length > 0 ? (
        <div
          className="flex flex-col gap-1 border-t border-line-soft pt-2"
          data-slot="subagent-hover-facts"
        >
          {facts.map(fact => (
            <PropertyRow
              key={fact.label}
              variant="facts"
              label={fact.label}
              mono={fact.kind === "mono" || fact.kind === "base"}
              valueTitle={hoverFactTitle(fact)}
            >
              {fact.kind === "mono" ? fact.value : <HoverFactValue fact={fact} />}
            </PropertyRow>
          ))}
        </div>
      ) : null}
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
