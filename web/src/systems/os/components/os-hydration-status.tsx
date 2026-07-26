import { CloudOff } from "lucide-react";

import { Icon, Pill } from "@agh/ui";

import type { OsHydration } from "../lib/os-types";

export interface OsHydrationStatusProps {
  hydration: OsHydration;
}

/** Non-blocking window-manager topology status for the system menubar. */
export function OsHydrationStatus({ hydration }: OsHydrationStatusProps) {
  if (hydration !== "degraded") return null;

  return (
    <Pill
      role="status"
      aria-atomic="true"
      aria-label="Desktop sync paused. Changes stay in memory while AGH reconnects."
      aria-live="polite"
      tone="warning"
      size="sm"
      className="shrink-0 gap-1.5"
    >
      <Icon as={CloudOff} size="sm" aria-hidden="true" />
      Sync paused · retrying
    </Pill>
  );
}
