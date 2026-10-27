import { ProviderUsageWindowSkeleton } from "@/systems/provider-usage/skeleton";

import { USAGE_WINDOW_FRAME } from "./usage-window-frame";

/**
 * The Usage window's body while its code loads: the content column and the panel's loading shape
 * (freshness line, Refresh, provider rows, Settings link) the window shows while its first read is
 * out, so the two are one continuous frame.
 */
export function UsageWindowFallback() {
  return (
    <div className={USAGE_WINDOW_FRAME}>
      <ProviderUsageWindowSkeleton className="flex-1" />
    </div>
  );
}
