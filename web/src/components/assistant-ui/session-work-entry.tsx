import { SessionToolCallRow, ThinkingBlock } from "@/systems/session";
import {
  useOptionalSessionNavigationTarget,
  useRevealHold,
} from "./hooks/session-navigation-target-context";
import { toolMessageFromPart } from "./session-timeline-tool-message";
import { isStreamingState, type SessionWorkEntry } from "./session-timeline.logic";

/** Render the original part; grouping never replaces its detail or copy actions. */
export function SessionWorkEntryView({
  entry,
  active,
  turnFailed,
}: {
  entry: SessionWorkEntry;
  active: boolean;
  turnFailed: boolean;
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
      interrupted={entry.status === "interrupted"}
      turnFailed={turnFailed}
      revealOpen={hold.held}
      onRevealRelease={hold.release}
      {...(hold.held && navigation?.reveal?.field ? { revealField: navigation.reveal.field } : {})}
    />
  );
}
