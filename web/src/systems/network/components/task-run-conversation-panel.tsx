import { AlertCircle } from "lucide-react";

import type { NetworkConversationMessage, TaskRunNetworkUsage } from "../types";
import { MessageTimeline } from "./timeline/timeline";

export interface TaskRunConversationPanelProps {
  messages: ReadonlyArray<NetworkConversationMessage>;
  usage: TaskRunNetworkUsage;
  conversationLoading?: boolean;
  conversationError?: Error | null;
  streamError?: Error | null;
  hasMoreMessages?: boolean;
  isFetchingMore?: boolean;
  onLoadMore?: () => void | Promise<void>;
  boundsLabel?: string | null;
}

const TASK_RUN_CONVERSATION_EMPTY_STATE = (
  <p className="text-sm text-muted" data-testid="tasks-run-conversation-empty">
    No coordination messages yet. Silence is normal until workers post updates on this run.
  </p>
);

/** Run-scoped conversation, bound consumption, and truthful usage surface (UT-057). */
export function TaskRunConversationPanel({
  messages,
  usage,
  conversationLoading = false,
  conversationError = null,
  streamError = null,
  hasMoreMessages = false,
  isFetchingMore = false,
  onLoadMore,
  boundsLabel,
}: TaskRunConversationPanelProps) {
  const total = usage.total;

  return (
    <section
      className="flex flex-col gap-3 rounded-md border border-border px-3 py-3"
      data-testid="tasks-run-conversation-panel"
    >
      <header className="flex flex-wrap items-baseline justify-between gap-3">
        <h2 className="text-sm font-medium text-foreground">Coordination conversation</h2>
        {boundsLabel ? (
          <p className="text-xs text-muted" data-testid="tasks-run-bounds-label">
            {boundsLabel}
          </p>
        ) : null}
      </header>

      {conversationError ? (
        <div className="flex items-center gap-2 text-sm text-danger" role="alert">
          <AlertCircle aria-hidden="true" className="size-4 shrink-0" />
          {conversationError.message}
        </div>
      ) : (
        <>
          {messages.length > 0 ? (
            <p className="text-xs text-muted" data-testid="tasks-run-conversation-summary">
              {messages.length} message{messages.length === 1 ? "" : "s"} loaded for this run.
            </p>
          ) : null}
          <MessageTimeline
            ariaLabel="Run coordination messages"
            asScrollContainer={false}
            className="rounded-md border border-border"
            density="overlay"
            emptyState={TASK_RUN_CONVERSATION_EMPTY_STATE}
            hasOlder={hasMoreMessages}
            isLoading={conversationLoading}
            isLoadingOlder={isFetchingMore}
            messages={messages}
            onLoadOlder={onLoadMore}
          />
        </>
      )}

      {streamError ? (
        <p className="text-xs text-warning" data-testid="tasks-run-conversation-stream-status">
          Live updates are reconnecting; paginated refresh remains active.
        </p>
      ) : null}

      <div
        className="border-t border-border pt-2 text-xs text-muted"
        data-testid="tasks-run-usage-summary"
      >
        Run usage: {total.actual_wake_count} actual · {total.reserved_wake_count} reserved ·{" "}
        {total.unavailable_wake_count} unavailable · {total.input_tokens} charged in /{" "}
        {total.output_tokens} charged out
      </div>
    </section>
  );
}
