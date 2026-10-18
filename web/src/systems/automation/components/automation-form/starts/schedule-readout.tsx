import { CircleCheck, TriangleAlert } from "lucide-react";

import { cn } from "@compozy/ui";

import type { ScheduleReadout as ScheduleReadoutModel } from "../../../lib/automation-form-schedule";

/** The plain-language line under the schedule builder; it carries its own glyph and tone. */
export function ScheduleReadout({ id, readout }: { id: string; readout: ScheduleReadoutModel }) {
  return (
    <output
      aria-live="polite"
      className={cn(
        "flex items-start gap-1.5 text-small-body leading-snug",
        readout.valid ? "text-fg-2" : "text-warning"
      )}
      data-testid="automation-schedule-readout"
      data-valid={readout.valid}
      id={id}
    >
      {readout.valid ? (
        <CircleCheck aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-success" />
      ) : (
        <TriangleAlert aria-hidden="true" className="mt-0.5 size-3.5 shrink-0" />
      )}
      <span>{readout.text}</span>
    </output>
  );
}
