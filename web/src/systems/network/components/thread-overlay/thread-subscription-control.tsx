import { useQuery } from "@tanstack/react-query";
import { Bell, BellOff, ChevronDown } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Spinner,
} from "@agh/ui";

import {
  useDeleteNetworkSubscription,
  useUpsertNetworkSubscription,
} from "../../hooks/use-network-actions";
import { networkSubscriptionsOptions } from "../../lib/query-options";
import type { NetworkSubscriptionMode } from "../../types";

const MODES: ReadonlyArray<{
  value: NetworkSubscriptionMode;
  label: string;
}> = [
  { value: "full", label: "Full delivery" },
  { value: "mute", label: "Mute" },
];

interface ThreadSubscriptionControlProps {
  workspaceId: string;
  channel: string;
  threadId: string;
  sessionId?: string | null;
}

function modeLabel(mode?: string | null): string {
  switch (mode) {
    case "full":
      return "Full";
    case "mute":
      return "Muted";
    default:
      return "Default";
  }
}

export function ThreadSubscriptionControl({
  workspaceId,
  channel,
  threadId,
  sessionId,
}: ThreadSubscriptionControlProps) {
  const enabled = workspaceId !== "" && channel !== "" && threadId !== "" && Boolean(sessionId);
  const {
    data: subscriptions = [],
    isError: hasSubscriptionError,
    isFetching: isSubscriptionsFetching,
  } = useQuery(
    networkSubscriptionsOptions(
      workspaceId,
      channel,
      { session_id: sessionId ?? undefined, thread_id: threadId },
      enabled
    )
  );
  const upsert = useUpsertNetworkSubscription();
  const remove = useDeleteNetworkSubscription();
  const current = subscriptions[0] ?? null;
  const isBusy = upsert.isPending || remove.isPending || isSubscriptionsFetching;
  const label = hasSubscriptionError ? "Unavailable" : modeLabel(current?.mode);
  const Icon = hasSubscriptionError || current?.mode === "mute" ? BellOff : Bell;
  const triggerLabel = hasSubscriptionError
    ? "Thread delivery mode unavailable"
    : "Thread delivery mode";

  const setMode = (mode: NetworkSubscriptionMode) => {
    if (!sessionId) {
      return;
    }
    void upsert.mutateAsync({
      workspaceId,
      channel,
      data: {
        session_id: sessionId,
        thread_id: threadId,
        mode,
      },
    });
  };

  const resetMode = () => {
    if (!sessionId) {
      return;
    }
    void remove.mutateAsync({ workspaceId, channel, sessionId, threadId });
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            aria-label={triggerLabel}
            data-testid="network-thread-subscription-trigger"
            disabled={!enabled || isBusy || hasSubscriptionError}
            size="sm"
            type="button"
            variant="ghost"
          />
        }
      >
        {isBusy ? (
          <Spinner aria-hidden="true" className="size-3" />
        ) : (
          <Icon aria-hidden="true" className="size-3" />
        )}
        {label}
        <ChevronDown aria-hidden="true" className="size-3 text-subtle" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {MODES.map(mode => (
          <DropdownMenuItem
            data-testid={`network-thread-subscription-${mode.value}`}
            key={mode.value}
            onSelect={event => {
              event.preventDefault();
              setMode(mode.value);
            }}
          >
            {mode.label}
          </DropdownMenuItem>
        ))}
        <DropdownMenuItem
          data-testid="network-thread-subscription-default"
          onSelect={event => {
            event.preventDefault();
            resetMode();
          }}
        >
          Default routing
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
