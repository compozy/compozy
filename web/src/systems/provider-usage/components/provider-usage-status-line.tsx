import { Button, cn, Spinner, StatusDot } from "@compozy/ui";

import { describeProviderUsageStatus, type ProviderUsageRowAction } from "../lib/status";
import type { ProviderUsageTarget } from "../types";

export interface ProviderUsageStatusLineProps {
  target: ProviderUsageTarget;
  now: number;
  /** The daemon host is macOS; **Allow** exists nowhere else. */
  isMacHost: boolean;
  /** This client reaches the daemon remotely; consent is approved on the host. */
  remoteClient?: boolean;
  /** Absent when the daemon is unreachable: the line keeps its reason, loses its button. */
  onAction?: (providerId: string, action: ProviderUsageRowAction) => void;
  /** An explicit action for this row is in flight. */
  pending?: boolean;
  /** Draw the tone dot; off where the row's collapsed cue already carries it. */
  dot?: boolean;
  className?: string;
}

/**
 * Why a row has no fresh numbers and the one thing to do about it. The dot
 * tone is backed by words; unsupported and disabled rows are text only.
 */
export function ProviderUsageStatusLine({
  target,
  now,
  isMacHost,
  remoteClient = false,
  onAction,
  pending = false,
  dot = true,
  className,
}: ProviderUsageStatusLineProps) {
  const status = describeProviderUsageStatus(target, { now, isMacHost, remoteClient });
  if (status === null) return null;
  const action = onAction ? status.action : null;
  return (
    <div
      data-slot="provider-usage-status-line"
      data-status={target.status}
      className={cn("flex min-w-0 flex-col gap-1", className)}
    >
      <div className="flex min-h-6 min-w-0 items-center gap-2">
        {dot && status.tone ? (
          <StatusDot tone={status.tone} variant={status.dotVariant} size="sm" />
        ) : null}
        <span className="min-w-0 flex-1 text-meta text-pretty text-muted-foreground">
          {status.text}
        </span>
        {action ? (
          <Button
            variant="outline"
            size="xs"
            aria-busy={pending || undefined}
            disabled={pending}
            onClick={() => onAction?.(target.provider_id, action.kind)}
          >
            {pending ? <Spinner className="size-3" aria-hidden="true" /> : null}
            {action.label}
          </Button>
        ) : null}
      </div>
      {status.note ? <p className="text-meta text-subtle">{status.note}</p> : null}
    </div>
  );
}
