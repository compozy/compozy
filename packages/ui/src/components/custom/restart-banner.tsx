"use client";

import { RefreshCwIcon, ShieldAlertIcon, XIcon } from "lucide-react";
import * as React from "react";

import { cn } from "../../lib/utils";
import { Alert, AlertDescription } from "../alert";
import { Button } from "../button";
import { Spinner } from "../spinner";

export type RestartBannerTone = "warning" | "info" | "success" | "danger";

export interface RestartBannerProps extends Omit<React.ComponentProps<"div">, "title" | "role"> {
  /** Visual tone. Defaults to `warning` (the warm-orange "Restart required to apply" chrome). */
  tone?: RestartBannerTone;
  /** Banner message. Defaults to "Restart required to apply." */
  message?: React.ReactNode;
  /** Optional inline detail chips rendered next to the message (operation id, active session count, …). */
  detail?: React.ReactNode;
  /** Renders the spinner instead of the shield-alert glyph. */
  busy?: boolean;
  /** Action callback for the inline "Restart daemon" button. When omitted, no action button renders. */
  restartNow?: () => void;
  /** Optional override label for the action button. Defaults to "Restart daemon". */
  actionLabel?: React.ReactNode;
  /** Disables the action button while a restart is in flight. */
  isPending?: boolean;
  /** Dismiss handler. When set, renders the inline dismiss button. */
  onDismiss?: () => void;
  /** Optional override label for the dismiss button. Defaults to "Dismiss". */
  dismissLabel?: React.ReactNode;
}

const ALERT_VARIANT: Record<RestartBannerTone, "warning" | "info" | "success" | "danger"> = {
  warning: "warning",
  info: "info",
  success: "success",
  danger: "danger",
};

const ALERT_ROLE: Record<RestartBannerTone, "alert" | "status"> = {
  warning: "status",
  info: "status",
  success: "status",
  danger: "alert",
};

function RestartBanner({
  tone = "warning",
  message,
  detail,
  busy = false,
  restartNow,
  actionLabel = "Restart daemon",
  isPending = false,
  onDismiss,
  dismissLabel = "Dismiss",
  className,
  ...props
}: RestartBannerProps) {
  return (
    <Alert
      variant={ALERT_VARIANT[tone]}
      role={ALERT_ROLE[tone]}
      data-slot="restart-banner"
      data-tone={tone}
      data-busy={busy ? "true" : undefined}
      data-pending={isPending ? "true" : undefined}
      className={cn(
        "flex flex-wrap items-center justify-between gap-3 rounded-none border-x-0 border-t-0 px-8 py-3 md:px-10",
        className
      )}
      {...props}
    >
      <div className="flex min-w-0 flex-1 items-center gap-2">
        {busy ? (
          <Spinner className="size-4 shrink-0" aria-hidden="true" data-slot="restart-banner-icon" />
        ) : (
          <ShieldAlertIcon
            className="size-4 shrink-0"
            aria-hidden="true"
            data-slot="restart-banner-icon"
          />
        )}
        <AlertDescription
          data-slot="restart-banner-message"
          className="flex min-w-0 flex-wrap items-center gap-2 text-sm"
        >
          <span data-slot="restart-banner-message-text" className="truncate">
            {message ?? "Restart required to apply."}
          </span>
          {detail ? <span data-slot="restart-banner-detail">{detail}</span> : null}
        </AlertDescription>
      </div>
      {restartNow || onDismiss ? (
        <div className="flex items-center gap-2">
          {restartNow ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              data-slot="restart-banner-action"
              disabled={isPending}
              onClick={restartNow}
            >
              <RefreshCwIcon className="size-3" />
              {isPending ? "Starting..." : actionLabel}
            </Button>
          ) : null}
          {onDismiss ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              data-slot="restart-banner-dismiss"
              onClick={onDismiss}
            >
              <XIcon className="size-3" />
              {dismissLabel}
            </Button>
          ) : null}
        </div>
      ) : null}
    </Alert>
  );
}

export { RestartBanner };
