import { cn } from "@agh/ui";

import type { SummarySegment, SummaryTone } from "../../../lib/trigger-preview";
import { PreviewCard } from "./preview-card";

const TONE_CLASS: Record<SummaryTone, string> = {
  plain: "text-muted",
  weak: "text-subtle font-medium",
  strong: "text-fg-strong font-semibold",
  accent: "text-accent-strong font-semibold",
  event: "rounded-xs bg-badge-fill px-1.5 py-px font-mono text-fg-strong",
};

interface PreviewSummaryProps {
  summary: SummarySegment[];
}

/** Plain-language "When … happens …, if …, run …" sentence. */
export function PreviewSummary({ summary }: PreviewSummaryProps) {
  return (
    <PreviewCard label="Summary">
      <p className="text-small-body leading-relaxed text-muted">
        {summary.map((segment, index) => (
          <span
            // Summary order is stable per render; index keys are safe here.
            key={`${segment.tone}-${index}`}
            className={cn(TONE_CLASS[segment.tone])}
          >
            {segment.text}
          </span>
        ))}
      </p>
    </PreviewCard>
  );
}
