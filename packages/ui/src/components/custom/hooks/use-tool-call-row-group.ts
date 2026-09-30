import * as React from "react";

export type ToolCallRowDensity = "default" | "inset";

export interface ToolCallRowGroupContextValue {
  density: ToolCallRowDensity;
  /** Hold every running glyph still (a paused window applies no live frames). */
  still: boolean;
}

const DEFAULT_CONTEXT: ToolCallRowGroupContextValue = { density: "default", still: false };

export const ToolCallRowGroupContext =
  React.createContext<ToolCallRowGroupContextValue>(DEFAULT_CONTEXT);

export function useToolCallRowGroup(): ToolCallRowGroupContextValue {
  return React.use(ToolCallRowGroupContext);
}
