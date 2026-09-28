"use client";

import { RefreshCwIcon, ShieldAlertIcon, XIcon } from "lucide-react";
import * as React from "react";

import { cn } from "../../lib/utils";
import { Alert, AlertDescription } from "../alert";
import { Button } from "../button";
import { Spinner } from "../spinner";

export type RestartBannerTone = "warning" | "info" | "success" | "danger";

type RestartBannerButtonProps = Omit<
  React.ComponentProps<typeof Button>,
  "onClick" | "children" | "disabled"
> & { "data-testid"?: string };

export interface RestartBannerProps extends Omit<React.ComponentProps<"div">, "title" | "role"> {
  /** Visual tone. Defaults to `warning` (the warm-orange "Restart required to apply" chrome). */
  tone?: RestartBannerTone;
  /** Banner message. Defaults to "Restart required to apply." */
  message?: React.ReactNode;
  /**
   * Optional second line under the message. When set, the message renders as a
   * title and wraps instead of truncating.
   */
  description?: React.ReactNode;
  /** Optional inline detail chips rendered next to the message (operation id, active session count, …). */
  detail?: React.ReactNode;
  /** Replaces the shield-alert glyph (ignored while `busy`). */
  icon?: React.ReactNode;
  /** Renders the spinner instead of the shield-alert glyph. */
  busy?: boolean;
  /** Action callback for the inline restart button. When omitted, no action button renders. */
  restartNow?: () => void;
  /** Optional override label for the action button. Defaults to "Restart now". */
  actionLabel?: React.ReactNode;
  /** Label shown on the action button while `isPending`. Defaults to "Starting...". */
  pendingLabel?: React.ReactNode;
  /** Disables the action button while a restart is in flight. */
  isPending?: boolean;
  /** Extra props for the action button (variant, test id, …). */
  actionProps?: RestartBannerButtonProps;
  /** Dismiss handler. When set, renders the inline dismiss button. */
  onDismiss?: () => void;
  /** Optional override label for the dismiss button. Defaults to "Dismiss". */
  dismissLabel?: React.ReactNode;
  /** Extra props for the dismiss button (test id, …). */
  dismissProps?: RestartBannerButtonProps;
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
  description,
  detail,
  icon,
  busy = false,
  restartNow,
  actionLabel = "Restart now",
  pendingLabel = "Starting...",
  isPending = false,
  actionProps,
  onDismiss,
  dismissLabel = "Dismiss",
  dismissProps,
  className,
  ...props
}: RestartBannerProps) {
  const hasDescription = description !== undefined && description !== null;
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
      <div
        className={cn("flex min-w-0 flex-1 gap-2", hasDescription ? "items-start" : "items-center")}
      >
        {busy ? (
          <Spinner
            className={cn("size-4 shrink-0", hasDescription && "mt-0.5")}
            aria-hidden="true"
            data-slot="restart-banner-icon"
          />
        ) : (
          <span
            aria-hidden="true"
            data-slot="restart-banner-icon"
            className={cn("flex shrink-0 [&_svg]:size-4", hasDescription && "mt-0.5")}
          >
            {icon ?? <ShieldAlertIcon />}
          </span>
        )}
        <AlertDescription
          data-slot="restart-banner-message"
          className={cn(
            "flex min-w-0 flex-wrap items-center gap-2 text-sm",
            hasDescription && "flex-col items-start gap-0.5"
          )}
        >
          <span
            data-slot="restart-banner-message-text"
            className={hasDescription ? "font-medium text-fg-strong" : "truncate"}
          >
            {message ?? "Restart required to apply."}
          </span>
          {hasDescription ? (
            <span data-slot="restart-banner-description">{description}</span>
          ) : null}
          {detail ? <span data-slot="restart-banner-detail">{detail}</span> : null}
        </AlertDescription>
      </div>
      {restartNow || onDismiss ? (
        <div className="flex items-center gap-2">
          {onDismiss ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              data-slot="restart-banner-dismiss"
              {...dismissProps}
              onClick={onDismiss}
            >
              <XIcon className="size-3" />
              {dismissLabel}
            </Button>
          ) : null}
          {restartNow ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              data-slot="restart-banner-action"
              aria-busy={isPending || undefined}
              {...actionProps}
              disabled={isPending}
              onClick={restartNow}
            >
              {isPending ? (
                <Spinner className="size-3" aria-hidden="true" />
              ) : (
                <RefreshCwIcon className="size-3" />
              )}
              {isPending ? pendingLabel : actionLabel}
            </Button>
          ) : null}
        </div>
      ) : null}
    </Alert>
  );
}

export { RestartBanner };
