import { Clock3, Radio } from "lucide-react";

import { PillGroup, type PillGroupItem } from "@compozy/ui";

type StartView = "all" | "schedule" | "event";

export interface AutomationStartViewCounts {
  all: number | null;
  schedule: number | null;
  event: number | null;
}

interface AutomationStartViewsProps {
  value: "schedule" | "event" | null;
  counts: AutomationStartViewCounts;
  onChange: (next: "schedule" | "event" | null) => void;
}

function CountText({ count }: { count: number | null }) {
  return <span className="font-mono font-normal tabular-nums text-subtle">{count ?? "—"}</span>;
}

/** Start views lead the toolbar: All · Scheduled · On events (webhooks count as events). */
export function AutomationStartViews({ value, counts, onChange }: AutomationStartViewsProps) {
  const items: PillGroupItem<StartView>[] = [
    {
      value: "all",
      label: (
        <>
          All <CountText count={counts.all} />
        </>
      ),
      testId: "automation-start-all",
    },
    {
      value: "schedule",
      label: (
        <>
          <Clock3 aria-hidden="true" className="size-3.5" />
          Scheduled <CountText count={counts.schedule} />
        </>
      ),
      testId: "automation-start-schedule",
    },
    {
      value: "event",
      label: (
        <>
          <Radio aria-hidden="true" className="size-3.5" />
          On events <CountText count={counts.event} />
        </>
      ),
      testId: "automation-start-event",
    },
  ];
  return (
    <PillGroup<StartView>
      aria-label="How it starts"
      data-testid="automation-start-views"
      items={items}
      onChange={next => onChange(next === "all" ? null : next)}
      size="sm"
      value={value ?? "all"}
    />
  );
}
