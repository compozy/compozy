"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

type IconComponent = React.ComponentType<{ className?: string; size?: number }>;

export type ActionResultBannerTone = "success" | "danger" | "warning" | "info" | "neutral";

export interface ActionResultBannerProps extends Omit<React.ComponentProps<"div">, "title"> {
  tone?: ActionResultBannerTone;
  title: React.ReactNode;
  description?: React.ReactNode;
  icon?: IconComponent;
  actions?: React.ReactNode;
}

const TONE_CHROME: Record<ActionResultBannerTone, string> = {
  success: "border-success-tint bg-success-tint text-success",
  danger: "border-danger-tint bg-danger-tint text-danger",
  warning: "border-warning-tint bg-warning-tint text-warning",
  info: "border-info-tint bg-info-tint text-info",
  neutral: "border-line bg-canvas-soft text-muted",
};

function ActionResultBanner({
  tone = "neutral",
  title,
  description,
  icon: Icon,
  actions,
  className,
  ...props
}: ActionResultBannerProps) {
  return (
    <div
      role="status"
      data-slot="action-result-banner"
      data-tone={tone}
      className={cn(
        "flex flex-wrap items-start gap-3 rounded-md border px-4 py-3",
        TONE_CHROME[tone],
        className
      )}
      {...props}
    >
      {Icon ? <Icon aria-hidden="true" className="size-3 shrink-0 mt-0.5" /> : null}
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <p data-slot="action-result-banner-title" className="text-small-body font-medium">
          {title}
        </p>
        {description ? (
          <p data-slot="action-result-banner-description" className="text-form-label opacity-90">
            {description}
          </p>
        ) : null}
      </div>
      {actions ? (
        <div data-slot="action-result-banner-actions" className="flex shrink-0 items-center gap-2">
          {actions}
        </div>
      ) : null}
    </div>
  );
}

export { ActionResultBanner };
