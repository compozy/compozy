import { TriangleAlert } from "lucide-react";

import { cn, Pill } from "@compozy/ui";

import { terminalConfidenceCopy } from "../lib/terminal-copy";
import type { TerminalDetectedBy } from "../types";

export function TerminalJournalConfidence({
  detectedBy,
  testId,
}: {
  detectedBy: TerminalDetectedBy;
  testId?: string;
}) {
  const confidence = terminalConfidenceCopy(detectedBy);
  return (
    <Pill
      className={cn(confidence.estimated && "gap-1")}
      data-testid={testId}
      size="xs"
      tone={confidence.estimated ? "warning" : "neutral"}
    >
      {confidence.estimated ? <TriangleAlert aria-hidden="true" className="size-2.5" /> : null}
      {confidence.label}
    </Pill>
  );
}
