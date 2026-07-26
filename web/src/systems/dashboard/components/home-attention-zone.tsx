import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";

import { Button, Panel, Section, StatusDot, Time } from "@agh/ui";

import { useHomeAttentionActions } from "../hooks/use-home-attention-actions";
import type { HomeAttentionResolvedKind } from "../hooks/use-home-attention-actions";
import type { HomeAttention, HomeAttentionItem } from "../types";

export interface HomeAttentionZoneProps {
  attention: HomeAttention;
}

interface HomeAttentionRowProps {
  item: HomeAttentionItem;
  resolved: HomeAttentionResolvedKind | undefined;
  onApprove: (taskId: string) => void;
  onReject: (taskId: string) => void;
  onRetry: (runId: string) => void;
  isMutating: boolean;
}

function attentionDotTone(kind: string): "warning" | "danger" {
  return kind === "failure" ? "danger" : "warning";
}

function attentionSentence(item: HomeAttentionItem): string {
  switch (item.kind) {
    case "approval":
      return "is waiting for your approval";
    case "failure":
      return item.detail ? `failed — ${item.detail}` : "failed";
    default:
      return item.detail ? `needs attention — ${item.detail}` : "needs your attention";
  }
}

function HomeAttentionRow({
  item,
  resolved,
  onApprove,
  onReject,
  onRetry,
  isMutating,
}: HomeAttentionRowProps) {
  if (resolved) {
    return (
      <div
        className="grid grid-cols-[14px_minmax(0,1fr)_auto] items-center gap-3 bg-success-tint px-4 py-3"
        data-resolved={resolved}
        data-slot="home-attention-row"
      >
        <StatusDot label="Resolved" tone="faint" />
        <span className="truncate text-small-body text-muted">
          {resolved === "approved" ? "Approved — " : "Rejected — "}
          <span className="font-medium text-fg-strong">{item.title}</span>
          {resolved === "approved" ? " is starting" : " will not run"}
        </span>
        <span className="font-mono text-mono-id tabular-nums text-subtle">now</span>
      </div>
    );
  }

  const taskId = item.task_id;
  const runId = item.run_id;
  return (
    <div
      className="grid grid-cols-[14px_minmax(0,1fr)_auto_auto] items-center gap-3 px-4 py-3 transition-colors duration-base hover:bg-row-hover max-[760px]:grid-cols-[14px_minmax(0,1fr)_auto]"
      data-slot="home-attention-row"
    >
      <StatusDot label={item.kind} tone={attentionDotTone(item.kind)} />
      <span className="truncate text-small-body text-muted max-[760px]:whitespace-normal">
        <span className="font-medium text-fg-strong">{item.title}</span> {attentionSentence(item)}
      </span>
      <span className="font-mono text-mono-id tabular-nums text-subtle max-[760px]:hidden">
        <Time iso={item.occurred_at} />
      </span>
      <span className="flex items-center gap-1.5">
        {item.actions.includes("approve") && taskId ? (
          <Button
            disabled={isMutating}
            onClick={() => onApprove(taskId)}
            size="sm"
            variant="primary"
          >
            Approve
          </Button>
        ) : null}
        {item.actions.includes("reject") && taskId ? (
          <Button disabled={isMutating} onClick={() => onReject(taskId)} size="sm" variant="ghost">
            Reject
          </Button>
        ) : null}
        {item.actions.includes("retry") && runId ? (
          <Button disabled={isMutating} onClick={() => onRetry(runId)} size="sm" variant="primary">
            Retry
          </Button>
        ) : null}
        {item.actions.includes("open") && taskId ? (
          <Button
            render={<Link params={{ id: taskId }} to="/tasks/$id" />}
            size="sm"
            variant="ghost"
          >
            Open
          </Button>
        ) : null}
      </span>
    </div>
  );
}

/**
 * Zone 1 — everything currently waiting on the user, with the daemon-accepted
 * verbs (approve/reject/retry/open) inline. Approve/reject resolve the row
 * optimistically; every settled decision reconciles the overview counters.
 */
export function HomeAttentionZone({ attention }: HomeAttentionZoneProps) {
  const { resolvedById, pendingIds, onApprove, onReject, onRetry } = useHomeAttentionActions();

  const inbox = (
    <Button render={<Link search={{ mode: "inbox" }} to="/tasks" />} size="sm" variant="ghost">
      Open inbox
      <ChevronRight aria-hidden="true" />
    </Button>
  );

  if (attention.total === 0 && attention.items.length === 0) {
    return (
      <Section count={0} label="Needs you" right={inbox}>
        <Panel bodyClassName="px-4 py-3.5">
          <p className="text-small-body text-subtle">Nothing needs you right now.</p>
        </Panel>
      </Section>
    );
  }

  return (
    <Section count={attention.total} label="Needs you" right={inbox}>
      <Panel bodyClassName="p-0" className="overflow-hidden">
        <div className="divide-y divide-line-soft">
          {attention.items.map(item => (
            <HomeAttentionRow
              isMutating={
                (item.task_id != null && pendingIds.has(item.task_id)) ||
                (item.run_id != null && pendingIds.has(item.run_id))
              }
              item={item}
              key={`${item.kind}:${item.task_id ?? item.title}`}
              onApprove={onApprove}
              onReject={onReject}
              onRetry={onRetry}
              resolved={item.task_id ? resolvedById[item.task_id] : undefined}
            />
          ))}
        </div>
      </Panel>
    </Section>
  );
}
