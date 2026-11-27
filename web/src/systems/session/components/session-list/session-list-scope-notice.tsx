import { Info, RefreshCw, WifiOff, X } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Button, cn } from "@compozy/ui";

import type { ComputerFixture } from "@/systems/computers/mocks/remote-computers-types";

function RailNotice({
  icon,
  title,
  body,
  action,
  testId,
}: {
  icon: ReactNode;
  title: string;
  body: string;
  action?: ReactNode;
  testId?: string;
}) {
  return (
    <div
      className="mx-2.5 mb-2 flex items-start gap-2.5 rounded-md border border-line bg-surface-2/60 px-3 py-2.5"
      data-testid={testId}
      role="status"
    >
      {icon}
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <p className="m-0 text-small-body text-fg">{title}</p>
        <p className="m-0 text-eyebrow text-muted-foreground">{body}</p>
      </div>
      {action}
    </div>
  );
}

/** A scope on an offline computer: its last known rows stay, dimmed, under one way back (`S8`). */
export function SessionListOfflineNotice({
  computer,
  onReconnect,
  reconnecting = false,
  testIdPrefix = "session-list",
}: {
  computer: Pick<ComputerFixture, "id" | "name" | "lastConnectedLabel">;
  onReconnect?: () => void;
  reconnecting?: boolean;
  testIdPrefix?: string;
}) {
  return (
    <RailNotice
      action={
        onReconnect ? (
          <Button
            className="h-6 shrink-0 px-2 text-badge"
            disabled={reconnecting}
            onClick={onReconnect}
            size="xs"
            type="button"
            variant="secondary"
          >
            <RefreshCw
              aria-hidden="true"
              className={cn("size-3", reconnecting && "animate-spin")}
            />
            Reconnect
          </Button>
        ) : undefined
      }
      body={`Last seen ${computer.lastConnectedLabel || "never"}. These sessions continue when it reconnects.`}
      icon={<WifiOff aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />}
      testId={`${testIdPrefix}-offline-notice`}
      title={`${computer.name} is offline`}
    />
  );
}

/** The scoped computer left the account or was disabled: falls back to all computers (`C2`, `C4`). */
export function SessionListScopeRemovedNotice({
  name,
  onDismiss,
  testIdPrefix = "session-list",
}: {
  name: string;
  onDismiss?: () => void;
  testIdPrefix?: string;
}) {
  return (
    <RailNotice
      action={
        onDismiss ? (
          <Button
            aria-label="Dismiss notice"
            className="size-6 shrink-0 text-subtle"
            onClick={onDismiss}
            size="icon-xs"
            type="button"
            variant="quiet"
          >
            <X aria-hidden="true" className="size-3" />
          </Button>
        ) : undefined
      }
      body="This window shows every computer now."
      icon={<Info aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />}
      testId={`${testIdPrefix}-scope-removed-notice`}
      title={`${name} was removed`}
    />
  );
}
