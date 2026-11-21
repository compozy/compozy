import { ReadonlyThreadProvider, ThreadPrimitive } from "@assistant-ui/react";
import { type ComponentProps, useId } from "react";

import { Button, Spinner, TranscriptDisclosure } from "@compozy/ui";
import {
  type SessionInheritedTranscriptState,
  type SessionInheritedView,
  useSessionInheritedTranscript,
} from "@/systems/session";
import { SessionRuntimeRenderProvider } from "@/systems/session/lib/session-runtime-render-context";
import { SessionThreadReadOnlyProvider } from "./session-thread-read-only-provider";

type MessageComponents = ComponentProps<typeof ThreadPrimitive.Unstable_MessageById>["components"];

const META_CLASS = "text-transcript-meta text-muted-foreground";

export interface SessionInheritedConversationProps {
  inherited: SessionInheritedView;
  /** The direct source: inherited rows resolve their attachments against it. */
  sourceSessionId: string;
  expanded: boolean;
  onToggle: () => void;
  /** The thread's own message renderer, reused read-only for the inherited rows. */
  messageComponents: MessageComponents;
}

/**
 * The conversation a continued or forked child inherited, above its origin
 * divider (#1885 C). Collapsed it is one counted sentence; expanded it reads the
 * source through the derive cut, tail first, and pages older rows on demand.
 * The rows sit on a hairline gutter, carry no fork/rewind actions, and are not
 * the child's: its search, outline and totals stay on its own messages.
 */
export function SessionInheritedConversation({
  inherited,
  sourceSessionId,
  expanded,
  onToggle,
  messageComponents,
}: SessionInheritedConversationProps) {
  const panelId = useId();
  const transcript = useSessionInheritedTranscript(inherited, expanded);
  return (
    <section
      aria-label="Earlier conversation"
      className="flex flex-col pt-2"
      data-expanded={expanded ? "true" : "false"}
      data-find-skip=""
      data-testid="session-inherited-conversation"
    >
      <TranscriptDisclosure
        aria-controls={expanded ? panelId : undefined}
        data-testid="session-inherited-toggle"
        expanded={expanded}
        icon={null}
        label={inherited.label}
        onToggle={onToggle}
        variant="turn"
      />
      {expanded ? (
        <div
          className="mt-2 flex flex-col gap-2 border-l border-line-soft pl-4"
          data-testid="session-inherited-panel"
          id={panelId}
        >
          {inherited.agentLine ? (
            <p className={META_CLASS} data-testid="session-inherited-agent-line">
              {inherited.agentLine}
            </p>
          ) : null}
          <InheritedBody
            messageComponents={messageComponents}
            sourceSessionId={sourceSessionId}
            transcript={transcript}
            workspaceId={inherited.workspaceId}
          />
        </div>
      ) : null}
    </section>
  );
}

function InheritedBody({
  transcript,
  sourceSessionId,
  workspaceId,
  messageComponents,
}: {
  transcript: SessionInheritedTranscriptState;
  sourceSessionId: string;
  workspaceId: string;
  messageComponents: MessageComponents;
}) {
  if (transcript.status === "error") {
    return (
      <div className="flex items-center gap-2" role="alert">
        <span className={META_CLASS}>Couldn't load the earlier conversation.</span>
        <Button onClick={transcript.retry} size="xs" type="button" variant="ghost">
          Try again
        </Button>
      </div>
    );
  }
  if (
    transcript.status === "unavailable" ||
    (transcript.status === "ready" && transcript.messages.length === 0)
  ) {
    return (
      <p className={META_CLASS} data-testid="session-inherited-unavailable" role="status">
        The source conversation is no longer available. The agent's carried context is unaffected.
      </p>
    );
  }
  if (transcript.status !== "ready") {
    return (
      <p className={`flex items-center gap-2 ${META_CLASS}`} role="status">
        <Spinner aria-hidden="true" />
        Loading the earlier conversation…
      </p>
    );
  }
  return (
    <>
      <InheritedOlderControl transcript={transcript} />
      <SessionRuntimeRenderProvider
        rewindBlocked
        sessionId={sourceSessionId}
        workspaceId={workspaceId}
      >
        <SessionThreadReadOnlyProvider readOnly>
          <ReadonlyThreadProvider messages={transcript.messages}>
            <div data-testid="session-inherited-messages">
              {transcript.messages.map(message => (
                <ThreadPrimitive.Unstable_MessageById
                  components={messageComponents}
                  key={message.id}
                  messageId={message.id}
                />
              ))}
            </div>
          </ReadonlyThreadProvider>
        </SessionThreadReadOnlyProvider>
      </SessionRuntimeRenderProvider>
    </>
  );
}

function InheritedOlderControl({ transcript }: { transcript: SessionInheritedTranscriptState }) {
  if (!transcript.hasOlder) return null;
  return (
    <div className="flex items-center gap-2">
      {transcript.olderFailed ? (
        <span className={META_CLASS} role="alert">
          Couldn't load earlier messages.
        </span>
      ) : null}
      <Button
        aria-busy={transcript.isFetchingOlder}
        data-testid="session-inherited-load-older"
        disabled={transcript.isFetchingOlder}
        onClick={transcript.loadOlder}
        size="xs"
        type="button"
        variant="ghost"
      >
        {transcript.isFetchingOlder ? <Spinner aria-hidden="true" /> : null}
        {transcript.olderFailed ? "Try again" : "Show earlier messages"}
      </Button>
    </div>
  );
}
