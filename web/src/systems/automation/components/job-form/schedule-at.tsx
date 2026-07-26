import { AlertTriangle, Check } from "lucide-react";

import { cn, Input } from "@agh/ui";

import { localInputToDate, toRfc3339 } from "../../lib/cron-engine";

interface ScheduleAtProps {
  time: string;
  valid: boolean;
  readout: string;
  onTime: (value: string) => void;
}

/**
 * One-time `at` schedule editor. Pure and presentational: the parent owns
 * validation and the human-readable {@link ScheduleAtProps.readout}; the ISO
 * label is derived inline from the timezone-naive input value.
 */
export function ScheduleAt({ time, valid, readout, onTime }: ScheduleAtProps) {
  const iso = toRfc3339(localInputToDate(time));

  return (
    <div>
      <div className="grid grid-cols-[1fr_auto] items-center gap-2.5">
        <Input
          aria-describedby="job-at-readout"
          aria-invalid={!valid}
          aria-label="Run date and time"
          onChange={event => onTime(event.target.value)}
          type="datetime-local"
          value={time}
        />
        <span className="font-mono text-form-hint whitespace-nowrap text-subtle">{iso}</span>
      </div>

      <output
        aria-live="polite"
        className={cn(
          "mt-2.5 flex items-center gap-1.5 text-small-body leading-snug",
          valid ? "text-success" : "text-danger"
        )}
        id="job-at-readout"
      >
        {valid ? (
          <Check aria-hidden="true" className="size-3 shrink-0" />
        ) : (
          <AlertTriangle aria-hidden="true" className="size-3 shrink-0" />
        )}
        <span>{readout}</span>
      </output>

      <p className="mt-3 text-form-hint leading-snug text-subtle">
        One-time run, then the job unregisters. If the time is already past when you save, it never
        registers.
      </p>
    </div>
  );
}
