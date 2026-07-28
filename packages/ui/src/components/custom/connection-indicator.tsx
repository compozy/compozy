"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import {
  ConnectionIndicatorContext,
  type ConnectionIndicatorContextValue,
} from "./hooks/use-connection-indicator-context";
import {
  ConnectionIndicatorDot,
  ConnectionIndicatorLabel,
  type ConnectionStatus,
  type ConnectionVariant,
} from "./connection-indicator-parts";

interface ConnectionIndicatorProps extends React.ComponentProps<"div"> {
  status: ConnectionStatus;
  label?: React.ReactNode;
  variant?: ConnectionVariant;
}

function ConnectionIndicator({
  status,
  label,
  variant = "footer",
  className,
  children,
  ...props
}: ConnectionIndicatorProps) {
  const value: ConnectionIndicatorContextValue = { label, status, variant };

  return (
    <ConnectionIndicatorContext.Provider value={value}>
      <div
        aria-live="polite"
        className={cn("inline-flex items-center gap-2", className)}
        data-slot="connection-indicator"
        data-status={status}
        data-variant={variant}
        role="status"
        {...props}
      >
        {children ??
          (variant === "rail-dot" ? (
            <ConnectionIndicatorDot />
          ) : (
            <>
              <ConnectionIndicatorDot />
              <ConnectionIndicatorLabel />
            </>
          ))}
      </div>
    </ConnectionIndicatorContext.Provider>
  );
}

const ConnectionIndicatorCompound = Object.assign(ConnectionIndicator, {
  Dot: ConnectionIndicatorDot,
  Label: ConnectionIndicatorLabel,
});

export { ConnectionIndicatorCompound as ConnectionIndicator };
export type {
  ConnectionIndicatorDotProps,
  ConnectionIndicatorLabelProps,
  ConnectionStatus,
  ConnectionVariant,
} from "./connection-indicator-parts";
export type { ConnectionIndicatorProps };
