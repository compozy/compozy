import { Sparkles, TriangleAlert, WifiOff, type LucideIcon } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Button, Icon } from "@compozy/ui";

import {
  ScopedComputerError,
  ScopedComputerSkeleton,
  type ComputerRead,
} from "@/systems/computers";

import { GroupNote } from "./session-list-group-note";

const SMALL_ACTION_CLASS = "h-6 shrink-0 px-2 text-badge";

export interface SessionListComputerNoteProps {
  icon?: LucideIcon;
  tone?: "subtle" | "warning";
  action?: ReactNode;
  children: ReactNode;
  testId?: string;
}

/** Inline computer status or refusal note inside a computer section (`F7`). */
export function SessionListComputerNote({
  icon,
  tone = "subtle",
  action,
  children,
  testId,
}: SessionListComputerNoteProps) {
  return (
    <GroupNote>
      <span className="flex min-w-0 flex-1 items-center gap-1.5" data-testid={testId}>
        {icon ? (
          <Icon
            as={icon}
            className={tone === "warning" ? "shrink-0 text-warning" : "shrink-0 text-faint"}
            size="sm"
          />
        ) : null}
        <span className="min-w-0 flex-1">{children}</span>
        {action}
      </span>
    </GroupNote>
  );
}

export interface SessionListComputerStateNoticeProps {
  computerId: string;
  name: string;
  read: ComputerRead;
  onRetry: () => void;
  onUpgrade?: () => void;
  onOpenDetails?: () => void;
  testIdPrefix: string;
}

/**
 * Non-row state bodies for a computer section (`S1`–`S6`):
 * connecting skeleton, failed with Retry, offline with no cached rows, Needs Pro with Upgrade,
 * or incompatible with Details. Returns `null` when the computer holds a live/cached row body.
 */
export function SessionListComputerStateNotice({
  computerId,
  name,
  read,
  onRetry,
  onUpgrade,
  onOpenDetails,
  testIdPrefix,
}: SessionListComputerStateNoticeProps) {
  switch (read) {
    case "connecting":
      return (
        <ScopedComputerSkeleton className="mx-1" computerId={computerId} computerName={name} />
      );
    case "failed":
      return (
        <ScopedComputerError
          className="mx-1"
          computerId={computerId}
          computerName={name}
          onRetry={onRetry}
        />
      );
    case "offline_empty":
      return (
        <SessionListComputerNote
          icon={WifiOff}
          testId={`${testIdPrefix}-computer-${computerId}-offline-empty`}
        >
          {name} is offline. Its sessions show when it reconnects.
        </SessionListComputerNote>
      );
    case "needs_pro":
      return (
        <SessionListComputerNote
          action={
            onUpgrade ? (
              <Button
                className={SMALL_ACTION_CLASS}
                onClick={onUpgrade}
                size="xs"
                type="button"
                variant="secondary"
              >
                Upgrade
              </Button>
            ) : undefined
          }
          icon={Sparkles}
          testId={`${testIdPrefix}-computer-${computerId}-needs-pro`}
        >
          Pro lists sessions from more than one remote computer.
        </SessionListComputerNote>
      );
    case "incompatible":
      return (
        <SessionListComputerNote
          action={
            onOpenDetails ? (
              <Button
                className={SMALL_ACTION_CLASS}
                onClick={onOpenDetails}
                size="xs"
                type="button"
                variant="secondary"
              >
                Details
              </Button>
            ) : undefined
          }
          icon={TriangleAlert}
          testId={`${testIdPrefix}-computer-${computerId}-incompatible`}
          tone="warning"
        >
          Update Compozy on {name} to list its sessions.
        </SessionListComputerNote>
      );
    default:
      return null;
  }
}
