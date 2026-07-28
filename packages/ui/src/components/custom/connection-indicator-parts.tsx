import * as React from "react";

import { cn } from "../../lib/utils";
import { useConnectionIndicatorContext } from "./hooks/use-connection-indicator-context";
import { Pill, type PillDotProps, type PillTone } from "./pill";

export type ConnectionStatus = "connected" | "connecting" | "disconnected" | "error";
export type ConnectionVariant = "footer" | "rail-dot" | "inline";

export interface ConnectionIndicatorDotProps extends Omit<PillDotProps, "tone" | "pulse"> {
  status?: ConnectionStatus;
}

export interface ConnectionIndicatorLabelProps extends React.ComponentProps<"span"> {
  status?: ConnectionStatus;
}

interface StatusConfig {
  tone: PillTone;
  label: string;
  pulse: boolean;
}

const STATUS_CONFIG: Record<ConnectionStatus, StatusConfig> = {
  connected: { tone: "success", label: "Connected", pulse: false },
  connecting: { tone: "warning", label: "Connecting", pulse: true },
  disconnected: { tone: "danger", label: "Disconnected", pulse: false },
  error: { tone: "danger", label: "Connection error", pulse: false },
};

export function ConnectionIndicatorDot({
  status,
  className,
  ...props
}: ConnectionIndicatorDotProps) {
  const context = useConnectionIndicatorContext(status);
  const config = STATUS_CONFIG[context.status];

  return (
    <Pill.Dot
      aria-hidden="true"
      className={className}
      data-slot="connection-indicator-dot"
      data-status={context.status}
      data-variant={context.variant}
      pulse={config.pulse}
      tone={config.tone}
      {...props}
    />
  );
}

export function ConnectionIndicatorLabel({
  status,
  className,
  children,
  ...props
}: ConnectionIndicatorLabelProps) {
  const context = useConnectionIndicatorContext(status);
  const config = STATUS_CONFIG[context.status];

  return (
    <span
      className={cn(
        context.variant === "inline"
          ? "font-sans text-form-label tracking-eyebrow text-muted"
          : "eyebrow text-muted",
        className
      )}
      data-slot="connection-indicator-label"
      data-status={context.status}
      data-variant={context.variant}
      {...props}
    >
      {children ?? context.label ?? config.label}
    </span>
  );
}
