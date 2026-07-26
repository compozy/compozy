import { Link, useNavigate } from "@tanstack/react-router";
import { ArrowUpRight, ListTodo, X } from "lucide-react";

import { Button, Eyebrow, MonoId, Pill } from "@agh/ui";

import { usePromoteNetworkThreadTask } from "../../hooks/use-network-actions";
import type { NetworkThreadDetail, NetworkThreadTaskLink } from "../../types";
import { ThreadSubscriptionControl } from "./thread-subscription-control";

export interface ThreadOverlayHeaderProps {
  workspaceId: string;
  channel: string;
  threadId: string;
  detail: NetworkThreadDetail | null;
  rootMessageId?: string | null;
  selfSessionId?: string | null;
  onClose: () => void;
  onOpenMain: () => void;
}

function buildParticipantLabel(detail: NetworkThreadDetail | null): string {
  if (!detail) {
    return "";
  }
  const participants = detail.participant_count ?? 0;
  if (participants <= 0) {
    return "no peers";
  }
  return participants === 1 ? "1 peer" : `${participants} peers`;
}

export function ThreadOverlayHeader({
  workspaceId,
  channel,
  threadId,
  detail,
  rootMessageId,
  selfSessionId,
  onClose,
  onOpenMain,
}: ThreadOverlayHeaderProps) {
  const navigate = useNavigate();
  const promote = usePromoteNetworkThreadTask();
  const title = detail?.title ?? "Thread";
  const participantLabel = buildParticipantLabel(detail);
  const originMessageId = rootMessageId ?? detail?.root_message_id ?? "";
  const canPromote = originMessageId.trim() !== "";

  const handlePromote = async () => {
    if (!canPromote) {
      return;
    }
    try {
      const result = await promote.mutateAsync({
        workspaceId,
        channel,
        threadId,
        data: {
          origin_message_id: originMessageId,
          title,
        },
      });
      void navigate({ params: { id: result.task.id }, to: "/tasks/$id" });
    } catch {
      // The mutation hook surfaces the toast; keep the operator on the thread.
    }
  };

  return (
    <header
      className="flex flex-wrap items-start gap-3 border-b border-line px-4 py-3"
      data-testid="network-thread-overlay-header"
    >
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <h2 className="min-w-0 truncate text-compact-h1 font-semibold tracking-compact-h1 text-fg-strong">
          {title}
        </h2>
        <div
          className="flex flex-wrap items-center gap-3 text-form-label text-muted"
          data-testid="network-thread-overlay-meta"
        >
          {participantLabel ? <Eyebrow className="text-subtle">{participantLabel}</Eyebrow> : null}
          <Button
            aria-label="Open thread in main pane"
            className="text-muted"
            data-testid="network-thread-overlay-open-main"
            onClick={onOpenMain}
            size="sm"
            type="button"
            variant="ghost"
          >
            <ArrowUpRight aria-hidden="true" className="size-3" />
            Open in main
          </Button>
        </div>
      </div>
      <div className="ml-auto flex shrink-0 items-center gap-2">
        <ThreadSubscriptionControl
          channel={channel}
          sessionId={selfSessionId}
          threadId={threadId}
          workspaceId={workspaceId}
        />
        <Button
          aria-label="Promote thread to task"
          data-testid="network-thread-promote-task"
          disabled={!canPromote || promote.isPending}
          onClick={() => void handlePromote()}
          size="sm"
          title="Create a task from this thread without starting a run."
          type="button"
          variant="neutral"
        >
          <ListTodo aria-hidden="true" className="size-3" />
          Promote
        </Button>
        <Button
          aria-label="Close thread overlay"
          data-testid="network-thread-overlay-close"
          onClick={onClose}
          size="icon-sm"
          type="button"
          variant="ghost"
        >
          <X aria-hidden="true" className="size-4" />
        </Button>
      </div>
    </header>
  );
}

export function ThreadTaskLinks({ links }: { links: ReadonlyArray<NetworkThreadTaskLink> }) {
  if (links.length === 0) {
    return null;
  }

  return (
    <div
      aria-label="Linked tasks"
      className="flex flex-wrap items-center gap-2 border-b border-line bg-canvas-soft px-4 py-2"
      data-testid="network-thread-task-links"
    >
      <Eyebrow className="text-subtle">Tasks</Eyebrow>
      {links.map(link => (
        <Pill.Link
          data-testid={`network-thread-task-link-${link.task_id}`}
          key={link.task_id}
          render={<Link params={{ id: link.task_id }} to="/tasks/$id" />}
        >
          <MonoId size="sm" value={link.task_id} />
        </Pill.Link>
      ))}
    </div>
  );
}
