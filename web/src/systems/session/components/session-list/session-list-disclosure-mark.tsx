import { ChevronRight } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Icon, StateGlyph, cn } from "@compozy/ui";

import { workspaceWorkingLabel } from "@/systems/workspace";

/** The hover/focus group a header's toggle names so its leading slot can swap. */
export const SESSION_LIST_DISCLOSURE_GROUP = "group/disclosure";

const SWAP = "transition-opacity duration-fast motion-reduce:transition-none";

/**
 * A group or folder header's leading 12 px slot. It holds the fold chevron, which
 * a collapsed header with work running yields at rest to its running signal: the
 * sidebar's one "work is running" column (#1926). Hover or keyboard focus on the
 * toggle brings the chevron back, as a session row's badge gives way to its checkbox.
 */
export function SessionListDisclosureMark({
  open,
  signal,
}: {
  open: boolean;
  /** Drawn only while the header is collapsed; an open header shows its rows instead. */
  signal?: ReactNode;
}) {
  const resting = open ? null : (signal ?? null);
  return (
    <span className="grid size-3 shrink-0 place-items-center *:col-start-1 *:row-start-1">
      <span
        className={cn(
          "grid place-items-center",
          resting !== null &&
            cn(
              SWAP,
              "opacity-0 group-hover/disclosure:opacity-100 group-focus-visible/disclosure:opacity-100"
            )
        )}
      >
        <Icon
          as={ChevronRight}
          size="sm"
          aria-hidden="true"
          className={cn("text-faint transition-transform", open && "rotate-90")}
        />
      </span>
      {resting !== null ? (
        <span
          className={cn(
            SWAP,
            "grid place-items-center group-hover/disclosure:opacity-0 group-focus-visible/disclosure:opacity-0"
          )}
        >
          {resting}
        </span>
      ) : null}
    </span>
  );
}

/** The leading running signal for `count` sessions working; none while nothing runs. */
export function sessionListRunningSignal(count: number, slot: string): ReactNode {
  if (count <= 0) return undefined;
  return (
    <span data-slot={slot} className="grid place-items-center">
      <StateGlyph size="sm" state="running" label={workspaceWorkingLabel(count)} />
    </span>
  );
}
