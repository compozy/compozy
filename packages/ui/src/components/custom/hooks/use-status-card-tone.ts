import * as React from "react";

import type { StatusCardTone } from "../status-card";

export const StatusCardContext = React.createContext<{ tone: StatusCardTone } | null>(null);

export function useStatusCardTone(tone?: StatusCardTone): StatusCardTone {
  const context = React.use(StatusCardContext);
  return tone ?? context?.tone ?? "neutral";
}
