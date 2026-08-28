import { ArrowDownLeft, ArrowUpRight } from "lucide-react";

import { Item, ItemActions, ItemContent, ItemMedia, ItemTitle, Time } from "@compozy/ui";

import { AgentCallStatePill } from "./agent-call-state-pill";
import { callRowWho } from "../lib/call-row-who";
import { toCallState } from "../lib/call-state";
import type { CallPayload } from "../types";

export function AgentCallsInspectorRow({
  call,
  direction,
  callerName,
  pruned,
  onOpenCall,
}: {
  call: CallPayload;
  direction: "made" | "received";
  callerName?: string;
  pruned: boolean;
  onOpenCall: (callId: string) => void;
}) {
  const Arrow = direction === "made" ? ArrowUpRight : ArrowDownLeft;
  const who = callRowWho(call, direction, callerName);
  const state = toCallState(call.state);
  return (
    <Item
      as="button"
      data-testid="agent-calls-panel-row"
      data-call-id={call.call_id}
      data-pruned={pruned || undefined}
      size="xs"
      selectable
      className="min-h-sidebar-row rounded-none px-0"
      aria-label={pruned ? `${who}, session pruned — record retained` : who}
      onClick={() => onOpenCall(call.call_id)}
    >
      <ItemMedia>
        <span className="inline-flex size-4.5 items-center justify-center rounded-mono-badge border border-line-soft bg-canvas-soft">
          <Arrow className="size-2.5 text-muted" aria-hidden="true" />
        </span>
      </ItemMedia>
      <ItemContent>
        <ItemTitle className="min-w-0 truncate font-normal text-form text-fg">{who}</ItemTitle>
      </ItemContent>
      <ItemActions>
        <AgentCallStatePill state={state} fallbackLabel={call.state} />
        {pruned ? (
          <span className="shrink-0 text-form text-muted">session pruned — record retained</span>
        ) : null}
        <Time iso={call.updated_at} mode="compact" className="shrink-0 text-form text-muted" />
      </ItemActions>
    </Item>
  );
}
