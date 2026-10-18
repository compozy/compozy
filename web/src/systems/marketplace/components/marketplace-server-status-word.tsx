import { cn, StatusDot, type StatusDotTone } from "@compozy/ui";

import type {
  MarketplaceServerStatusTone,
  MarketplaceServerStatusView,
} from "./marketplace-server-status";

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

/**
 * Status word + 6px dot (hollow when neutral): the tone rides the dot only, the word stays muted
 * ink, so a state never reads as a control or a second accent.
 */
function MarketplaceServerStatusWord({
  view,
  className,
  "data-testid": testId,
}: MarketplaceServerStatusWordProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 text-eyebrow font-medium whitespace-nowrap text-muted",
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
