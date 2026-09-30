import {
  SessionToolCallRow,
  liveToolLabel,
  parallelToolLabel,
  getToolIcon,
  resolveRegisteredToolName,
} from "@/systems/session";
import type { SessionNavigationReveal } from "./hooks/session-navigation-target-context";
import { toolMessageFromPart } from "./session-timeline-tool-message";

import { useSessionThreadLiveData } from "./hooks/use-session-thread-live-data";
import { StateGlyph, ToolCallRow, TranscriptDisclosure } from "@compozy/ui";

import type { SessionLiveToolRow, SessionTimelineToolPart } from "./session-timeline.logic";

// The one live line (ADR-006 rule 1), in the sunken tool panel's grammar: kind
// glyph, the active verb, the mono preview, and the mint running ring as the
// row's only motion. A paused window (or reduced motion) holds the ring still;
// the verb carries the state. Plain text, not a popover: the raw input is one
// click away once the call settles into its tool row; a truncated title keeps
// the full name on hover.
function LiveToolLine({ part }: { part: SessionTimelineToolPart }) {
  const label = liveToolLabel(part.toolName, part.args, part.toolTitle);
  return (
    <ToolCallRow
      data-testid="live-tool-row"
      data-live-kind={part.toolName}
      icon={getToolIcon(resolveRegisteredToolName(part.toolName), part.args)}
      toolName={
        <span data-testid="live-tool-label" title={part.toolTitle ?? label.text}>
          {label.verb}
        </span>
      }
      preview={label.preview ?? undefined}
      status="running"
    />
  );
}

export interface SessionLiveToolRowViewProps {
  row: SessionLiveToolRow;
  reducedMotion: boolean;
  onToggle: () => void;
  reveal?: SessionNavigationReveal | null;
}

/**
 * `SessionLiveToolRow` (ADR-006 rule 1): exactly one live row for the calls
 * still running, set in the sunken tool panel. A single call reads "{verb}
 * {preview}"; several stay one honest row — "Running N tools…" — that expands
 * to the in-flight list. A running child agent is its own row with the bot
 * glyph. The running ring is the row's only motion: it holds still under
 * reduced motion and while the window is paused (US-018.EC-2) — a view that is
 * not applying frames has no cadence to show.
 */
export function SessionLiveToolRowView({
  row,
  reducedMotion,
  onToggle,
  reveal,
}: SessionLiveToolRowViewProps) {
  const liveData = useSessionThreadLiveData();
  const still = reducedMotion || !liveData;
  const first = row.entries[0];
  if (!first) return null;
  if (row.entries.length === 1) {
    return (
      <div
        data-testid="live-tool"
        data-agent={row.agent || undefined}
        data-still={still || undefined}
        className="flex min-w-0 flex-col"
      >
        <ToolCallRow.Group still={still}>
          <LiveToolLine part={first} />
        </ToolCallRow.Group>
        {reveal ? (
          <SessionToolCallRow
            message={toolMessageFromPart(first)}
            partIndex={first.partIndex}
            revealOpen
            revealField={reveal.field ?? undefined}
            onRevealRelease={onToggle}
          />
        ) : null}
      </div>
    );
  }
  const detailsId = `${row.id}:entries`;
  return (
    <div
      data-testid="live-tool"
      data-parallel={row.entries.length}
      data-still={still || undefined}
      className="flex min-w-0 flex-col"
    >
      <TranscriptDisclosure
        aria-controls={detailsId}
        data-testid="live-tool-parallel"
        expanded={row.expanded}
        icon={<StateGlyph size="sm" state="running" still={still} />}
        label={
          <span className="font-medium" data-testid="live-tool-label">
            {parallelToolLabel(row.entries.length)}
          </span>
        }
        onToggle={onToggle}
      />
      <div
        id={detailsId}
        data-testid="live-tool-entries"
        hidden={!row.expanded}
        aria-hidden={!row.expanded}
        inert={!row.expanded}
        className={row.expanded ? "min-w-0 pt-1.5" : undefined}
      >
        {row.expanded ? (
          <ToolCallRow.Group still={still}>
            {row.entries.map(part => (
              <div key={part.id} className="min-w-0">
                <LiveToolLine part={part} />
                {reveal && reveal.partIndex === part.partIndex ? (
                  <SessionToolCallRow
                    message={toolMessageFromPart(part)}
                    partIndex={part.partIndex}
                    revealOpen
                    revealField={reveal.field ?? undefined}
                    onRevealRelease={onToggle}
                  />
                ) : null}
              </div>
            ))}
          </ToolCallRow.Group>
        ) : null}
      </div>
    </div>
  );
}
