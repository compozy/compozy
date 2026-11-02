import { PermissionDataPart, SessionToolCallRow, ThinkingBlock } from "@/systems/session";
import {
  useOptionalSessionNavigationTarget,
  useRevealHold,
} from "./hooks/session-navigation-target-context";
import { isCompozyPermissionData } from "./session-timeline-permissions";
import { toolMessageFromPart } from "./session-timeline-tool-message";
import {
  isStreamingState,
  isInterruptedState,
  type SessionTimelineDataPart,
  type SessionWorkApprovals,
  type SessionWorkEntry,
} from "./session-timeline.logic";

function SessionWorkApprovalReceipt({ part }: { part: SessionTimelineDataPart }) {
  if (!isCompozyPermissionData(part.data)) return null;
  return (
    <div data-part-index={part.partIndex} className="min-w-0">
      <PermissionDataPart data={part.data} />
    </div>
  );
}

/** Render the original part; grouping never replaces its detail or copy actions. */
export function SessionWorkEntryView({
  entry,
  approvals,
  active,
  turnFailed,
  disclosed = false,
  onRail = false,
}: {
  entry: SessionWorkEntry;
  /** Receipts of the row's calls; this entry draws its own under the tool. */
  approvals?: SessionWorkApprovals;
  active: boolean;
  turnFailed: boolean;
  disclosed?: boolean;
  /** The entry hangs off a work group's rail: the branch is its bullet. */
  onRail?: boolean;
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
    <>
      <SessionToolCallRow
        message={toolMessageFromPart(entry)}
        partIndex={entry.partIndex}
        turnSettled={!active}
        interrupted={entry.status === "interrupted" || isInterruptedState(entry.state)}
        turnFailed={turnFailed}
        onRail={onRail}
        revealOpen={hold.held}
        onRevealRelease={hold.release}
        {...(hold.held && navigation?.reveal?.field
          ? { revealField: navigation.reveal.field }
          : {})}
      />
      {approvals?.get(entry.toolCallId)?.map(part => (
        <SessionWorkApprovalReceipt key={part.id} part={part} />
      ))}
    </>
  );
}
