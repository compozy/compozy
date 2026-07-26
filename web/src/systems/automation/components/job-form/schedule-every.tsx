import { AlertTriangle, Check } from "lucide-react";

import { cn, Input } from "@agh/ui";

interface ScheduleEveryProps {
  interval: string;
  valid: boolean;
  readout: string;
  onInterval: (value: string) => void;
  onPreset: (value: string) => void;
}

const EVERY_PRESETS = ["5m", "15m", "30m", "1h", "4h", "12h", "24h"];

const CHIP_BASE =
  "inline-flex items-center rounded-pill border px-2.5 py-1 font-mono text-form-label font-medium transition-colors outline-none focus-visible:shadow-focus-ring";
const CHIP_RESTING = "border-line-soft bg-canvas-tint text-muted hover:bg-elevated hover:text-fg";
const CHIP_SELECTED = "border-accent-dim bg-accent-tint text-accent-strong";

/**
 * Fixed-interval schedule editor. Pure and presentational: the parent owns
 * validation and the human-readable {@link ScheduleEveryProps.readout}.
 */
export function ScheduleEvery({
  interval,
  valid,
  readout,
  onInterval,
  onPreset,
}: ScheduleEveryProps) {
  return (
    <div>
      <div className="mb-3 flex flex-wrap gap-1.5" aria-label="Common intervals">
        {EVERY_PRESETS.map(preset => {
          const pressed = preset === interval;
          return (
            <button
              key={preset}
              aria-pressed={pressed}
              className={cn(CHIP_BASE, pressed ? CHIP_SELECTED : CHIP_RESTING)}
              onClick={() => onPreset(preset)}
              type="button"
            >
              {preset}
            </button>
          );
        })}
      </div>

      <Input
        aria-describedby="job-every-readout"
        aria-invalid={!valid}
        aria-label="Interval"
        className="font-mono"
        onChange={event => onInterval(event.target.value)}
        placeholder="30m"
        value={interval}
      />

      <output
        aria-live="polite"
        className={cn(
          "mt-2.5 flex items-center gap-1.5 text-small-body leading-snug",
          valid ? "text-success" : "text-danger"
        )}
        id="job-every-readout"
      >
        {valid ? (
          <Check aria-hidden="true" className="size-3 shrink-0" />
        ) : (
          <AlertTriangle aria-hidden="true" className="size-3 shrink-0" />
        )}
        <span>{readout}</span>
      </output>

      <p className="mt-3 text-form-hint leading-snug text-subtle">
        Positive Go duration: <code className="font-mono text-mono-id">30m</code>,{" "}
        <code className="font-mono text-mono-id">1h</code>,{" "}
        <code className="font-mono text-mono-id">2h30m</code>,{" "}
        <code className="font-mono text-mono-id">45s</code>.
      </p>
    </div>
  );
}
