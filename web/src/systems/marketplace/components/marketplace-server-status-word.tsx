import { cn, StatusDot, type StatusDotTone } from "@compozy/ui";

import type {
  MarketplaceServerStatusTone,
  MarketplaceServerStatusView,
} from "./marketplace-server-status";

const TONE_TEXT: Record<MarketplaceServerStatusTone, string> = {
  success: "text-success",
  warning: "text-warning",
  neutral: "text-subtle",
};

const TONE_DOT: Record<MarketplaceServerStatusTone, StatusDotTone> = {
  success: "success",
  warning: "warning",
  neutral: "faint",
};

interface MarketplaceServerStatusWordProps {
  view: MarketplaceServerStatusView;
  className?: string;
  "data-testid"?: string;
}

/** Status word + 6px dot (hollow when neutral). Tone marks state only. */
function MarketplaceServerStatusWord({
  view,
  className,
  "data-testid": testId,
}: MarketplaceServerStatusWordProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 text-eyebrow font-medium whitespace-nowrap",
        TONE_TEXT[view.tone],
        className
      )}
      data-status={view.key}
      data-testid={testId}
    >
      <StatusDot
        aria-hidden="true"
        size="sm"
        tone={TONE_DOT[view.tone]}
        variant={view.tone === "neutral" ? "ring" : "solid"}
      />
      {view.label}
    </span>
  );
}

export { MarketplaceServerStatusWord };
export type { MarketplaceServerStatusWordProps };
