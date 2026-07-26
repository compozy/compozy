import { Separator } from "@agh/ui";

import { formatDatePill } from "../../lib/format-timestamp";

export interface DatePillProps {
  timestamp: string;
  /** Optional reference moment, primarily for tests. */
  now?: Date;
}

export function DatePill({ timestamp, now }: DatePillProps) {
  const label = formatDatePill(timestamp, { now });
  if (!label) {
    return null;
  }

  return (
    <Separator
      className="my-6 px-5"
      data-testid="network-timeline-date-pill"
      data-label={label}
      label={label}
    />
  );
}
