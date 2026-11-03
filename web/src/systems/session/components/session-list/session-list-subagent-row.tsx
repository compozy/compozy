import type { MouseEvent } from "react";

import { StateGlyph, cn } from "@compozy/ui";

import { useSubagentDrawerSelected } from "../../hooks/use-subagent-drawer-selected";
import { subagentDisplay } from "../subagents/subagent-format";
import { SubagentRowTrail } from "../subagents/subagent-row-trail";
import { SubagentRuntimeMeta } from "../subagents/subagent-runtime-meta";
import type { SubagentView } from "../subagents/types";
import { BRANCH_NODE } from "./session-list-branch-rail";
import { SUBAGENT_ROW_FRAME } from "./session-list-row-frame";

/** One direct subagent on its parent's rail; `SessionListSubagentBranch` pages and opens them. */
export function SessionListSubagentRow({
  subagent,
  current,
  onOpen,
}: {
  subagent: SubagentView;
  current: boolean;
  onOpen?: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  const selected = useSubagentDrawerSelected(subagent.id);
  const display = subagentDisplay(subagent);
  const word = display.word;
  const body = (
    <>
      {/* The mark sits on the title line, where the branch elbow lands, not between both lines. */}
      <span className="flex h-5 items-center justify-center self-start">
        <StateGlyph size="sm" state={display.glyph} />
      </span>
      <span className="flex min-w-0 flex-col">
        <span className={cn("truncate", display.settled && "text-muted-foreground")}>
          {subagent.title}
        </span>
        <SubagentRuntimeMeta subagent={subagent} />
      </span>
      <SubagentRowTrail subagent={subagent} />
    </>
  );
  const rowClass = cn(SUBAGENT_ROW_FRAME, (current || selected) && "bg-selected");

  return (
    <div
      className={BRANCH_NODE}
      data-slot="session-subagent-row"
      data-status={subagent.status}
      data-state={display.glyph}
    >
      {onOpen ? (
        <button
          type="button"
          className={cn(
            rowClass,
            "transition-colors hover:bg-surface-2 focus-visible:shadow-focus-ring"
          )}
          aria-label={`Open ${subagent.title}`}
          aria-description={word}
          aria-current={current || selected ? "true" : undefined}
          onClick={onOpen}
        >
          {body}
        </button>
      ) : (
        <div className={rowClass}>{body}</div>
      )}
    </div>
  );
}
