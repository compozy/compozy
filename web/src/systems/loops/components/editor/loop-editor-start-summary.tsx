import type { LoopStartBinding } from "../../types";
import { Eyebrow, Pill } from "@compozy/ui";

interface LoopEditorStartSummaryProps {
  start: LoopStartBinding[];
}

export function LoopEditorStartSummary({ start }: LoopEditorStartSummaryProps) {
  if (start.length === 0) return null;
  return (
    <div
      className="pointer-events-none absolute left-3.5 top-3 z-10 flex items-center gap-1.5 rounded-md bg-sunken px-2.5 py-1.5"
      data-testid="loop-editor-start-summary"
      title="How this Loop can start. Change this in the Loop file."
    >
      <Eyebrow variant="caps" className="text-faint">
        Starts
      </Eyebrow>
      {start.map(binding => (
        <Pill key={JSON.stringify(binding)} size="xs" tone="neutral" mono>
          {binding.kind}
        </Pill>
      ))}
    </div>
  );
}
