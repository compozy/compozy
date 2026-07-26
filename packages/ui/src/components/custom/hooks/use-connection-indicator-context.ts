import * as React from "react";

import type { ConnectionStatus, ConnectionVariant } from "../connection-indicator";

export interface ConnectionIndicatorContextValue {
  label?: React.ReactNode;
  status: ConnectionStatus;
  variant: ConnectionVariant;
}

export const ConnectionIndicatorContext =
  React.createContext<ConnectionIndicatorContextValue | null>(null);

export function useConnectionIndicatorContext(
  status?: ConnectionStatus
): ConnectionIndicatorContextValue {
  const context = React.use(ConnectionIndicatorContext);
  if (status !== undefined) return { label: undefined, status, variant: "footer" };
  if (context) return context;
  return { label: undefined, status: "disconnected", variant: "footer" };
}
