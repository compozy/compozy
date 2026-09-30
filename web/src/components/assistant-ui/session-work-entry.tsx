import type { ReactNode } from "react";

import { rendersTerminalBlock, SessionToolCallRow, ThinkingBlock } from "@/systems/session";
import { ToolCallRow } from "@compozy/ui";
import {
  useOptionalSessionNavigationTarget,
  useRevealHold,
} from "./hooks/session-navigation-target-context";
import { toolMessageFromPart } from "./session-timeline-tool-message";
import {
  isStreamingState,
  isInterruptedState,
  type SessionWorkEntry,
} from "./session-timeline.logic";

/** Render the original part; grouping never replaces its detail or copy actions. */
export function SessionWorkEntryView({
  entry,
  active,
  turnFailed,
  disclosed = false,
}: {
  entry: SessionWorkEntry;
  active: boolean;
  turnFailed: boolean;
  disclosed?: boolean;
}) {
  const navigation = useOptionalSessionNavigationTarget();
  const id = entry.kind === "tool" ? `tool:${entry.toolCallId}` : `reasoning:${entry.id}`;
  const hold = useRevealHold(
    id,
    reveal =>
      reveal.partIndex !== null &&
      entry.partIndex === reveal.partIndex &&
      (entry.kind === "reasoning" || reveal.opensBody)
  );
  if (entry.kind === "reasoning") {
    return (
      <ThinkingBlock
        thinking={entry.text}
        defaultOpen={disclosed}
        thinkingComplete={!isStreamingState(entry.state)}
        partIndex={entry.partIndex}
        revealOpen={hold.held}
        onRevealRelease={hold.release}
      />
    );
  }
  return (
    <SessionToolCallRow
      message={toolMessageFromPart(entry)}
      partIndex={entry.partIndex}
      turnSettled={!active}
      interrupted={entry.status === "interrupted" || isInterruptedState(entry.state)}
      turnFailed={turnFailed}
      revealOpen={hold.held}
      onRevealRelease={hold.release}
      {...(hold.held && navigation?.reveal?.field ? { revealField: navigation.reveal.field } : {})}
    />
  );
}

function entryKey(entry: SessionWorkEntry): string {
  return `${entry.kind}:${entry.id}`;
}

// A tool call that renders as a tool line (not reasoning, not a live terminal block).
function isToolLine(entry: SessionWorkEntry): boolean {
  return entry.kind === "tool" && !rendersTerminalBlock(toolMessageFromPart(entry));
}

/**
 * An ordered run of work entries: consecutive tool lines share one sunken tool
 * panel (`ToolCallRow.Group`), while reasoning and terminal blocks sit between
 * panels at the transcript level, so the order of the turn is kept.
 */
export function SessionWorkEntryList({
  entries,
  active,
  turnFailed,
  disclosed = false,
}: {
  entries: readonly SessionWorkEntry[];
  active: boolean;
  turnFailed: boolean;
  disclosed?: boolean;
}) {
  const nodes: ReactNode[] = [];
  let panel: SessionWorkEntry[] = [];
  let panelCount = 0;
  const flush = () => {
    if (panel.length === 0) return;
    // Keyed by ordinal, not by its first entry: a call that lands at the head of
    // a panel must not remount the rows (and focus) already inside it.
    nodes.push(
      <ToolCallRow.Group key={`panel:${panelCount}`} data-testid="work-tool-panel">
        {panel.map(entry => (
          <SessionWorkEntryView
            key={entryKey(entry)}
            entry={entry}
            active={active}
            turnFailed={turnFailed}
            disclosed={disclosed}
          />
        ))}
      </ToolCallRow.Group>
    );
    panelCount += 1;
    panel = [];
  };
  for (const entry of entries) {
    if (isToolLine(entry)) {
      panel.push(entry);
      continue;
    }
    flush();
    nodes.push(
      <SessionWorkEntryView
        key={entryKey(entry)}
        entry={entry}
        active={active}
        turnFailed={turnFailed}
        disclosed={disclosed}
      />
    );
  }
  flush();
  return nodes;
}
