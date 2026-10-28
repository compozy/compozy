import type { ThreadMessage } from "@assistant-ui/react";
import { SessionSubagentsProvider, type SubagentView } from "@/systems/session";
import { useEffect } from "react";
import { useSelector, useStore } from "@xstate/store-react";
import { Button } from "@compozy/ui";
import { useSessionTranscriptThreadState } from "@/systems/session/hooks/use-session-transcript-thread-messages";
import { toTimelineParts } from "@/systems/session/lib/timeline-message-parts";
import { SessionThreadReadOnlyProvider } from "./session-thread-read-only-provider";
import { TimelineRowContext, timelineRowLogic } from "./hooks/use-timeline-row-context";
import { subagentPartData } from "./session-timeline-subagents";
import { deriveSessionRows } from "./session-timeline.logic";
import { SessionMessageText } from "./session-message-parts";
import { SessionTimelineRows } from "./session-timeline-render";

/** Native activity belongs to a tool call in the parent ledger, never a fabricated session. */
export function SessionNativeSubagentActivity({
  subagent,
  workspaceId,
}: {
  subagent: SubagentView;
  workspaceId: string;
}) {
  const transcript = useSessionTranscriptThreadState();
  const store = useStore(timelineRowLogic, undefined);
  const expansion = useSelector(store, snapshot => snapshot.context);
  const { prompt, activity, hasInvocation } = nativeActivity(transcript.messages, subagent.id);
  const { hasOlder, isFetchingOlder, isError, loadOlder } = transcript;
  useEffect(() => {
    if (!hasInvocation && hasOlder && !isFetchingOlder && !isError) void loadOlder();
  }, [hasInvocation, hasOlder, isFetchingOlder, isError, loadOlder]);
  const rows = deriveSessionRows(activity, {
    expandedWorkGroupIds: expansion.expandedWorkGroups,
    expandedChangedFilesIds: expansion.expandedChangedFiles,
  });
  return (
    <SessionSubagentsProvider workspaceId={workspaceId} sessionId={subagent.parent_session_id}>
      <SessionThreadReadOnlyProvider readOnly>
        <TimelineRowContext value={store}>
          <div
            className="flex min-h-0 flex-1 flex-col gap-3 overflow-auto p-4"
            data-testid="native-subagent-activity"
          >
            {prompt ? <SessionMessageText text={prompt} authored /> : null}
            <NativeOutcome subagent={subagent} />
            <NativeActivityRows
              transcript={transcript}
              rows={rows}
              hasResult={Boolean(subagent.result || subagent.error)}
            />
          </div>
        </TimelineRowContext>
      </SessionThreadReadOnlyProvider>
    </SessionSubagentsProvider>
  );
}

function nativeActivity(messages: readonly ThreadMessage[], subagentId: string) {
  const parts = messages.flatMap(toTimelineParts);
  const owner = parts.map(subagentPartData).find(part => part?.subagent_id === subagentId);
  const toolId = owner?.tool_call_id;
  const call = parts.find(part => part.kind === "tool" && part.toolCallId === toolId);
  const prompt =
    call?.kind === "tool" && typeof call.args.prompt === "string" ? call.args.prompt : null;
  const activity = toolId
    ? parts.filter(
        part =>
          part.parentToolCallId === toolId || (part.kind === "tool" && part.toolCallId === toolId)
      )
    : [];
  return { prompt, activity, hasInvocation: call !== undefined };
}

function NativeOutcome({ subagent }: { subagent: SubagentView }) {
  return (
    <>
      {subagent.result ? <SessionMessageText text={subagent.result} /> : null}
      {subagent.result_truncated ? (
        <p className="text-small-body text-muted">Result was truncated.</p>
      ) : null}
      {subagent.error ? (
        <p role="alert" className="text-small-body text-danger">
          {subagent.error}
        </p>
      ) : null}
    </>
  );
}

function NativeActivityRows({
  transcript,
  rows,
  hasResult,
}: {
  transcript: ReturnType<typeof useSessionTranscriptThreadState>;
  rows: ReturnType<typeof deriveSessionRows>;
  hasResult: boolean;
}) {
  if (transcript.isError)
    return (
      <div role="alert">
        Couldn't load subagent activity.{" "}
        <Button variant="ghost" onClick={transcript.retry}>
          Try again
        </Button>
      </div>
    );
  if (rows.length) return <SessionTimelineRows rows={rows} />;
  if (hasResult) return null;
  return (
    <p role="status" className="text-small-body text-muted">
      {transcript.isPending || transcript.isFetchingOlder
        ? "Loading activity…"
        : "No activity has been reported yet."}
    </p>
  );
}
