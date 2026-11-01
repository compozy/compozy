/** What the agent has actually done: findings first, then the running log. */
import { protoHref } from "@compozy/proto";
import { Check } from "lucide-react";

import { Panel, Pill, Section, Timeline, TimelineEvent, cn } from "@compozy/ui";

import {
  HOME_RUN_ROW_CLASS,
  HOME_RUN_SUBTITLE_CLASS,
  HOME_RUN_TITLE_CLASS,
} from "@/systems/dashboard/components/home-zone-shape";

import type { Finding, ProfileData, WorkItem } from "./profile-fixtures";

function openReport() {
  window.location.assign(protoHref("stays-on-reports", "proposal"));
}

function FindingRow({ finding }: { finding: Finding }) {
  return (
    <button
      aria-label={`Open: ${finding.title}`}
      className={cn(
        HOME_RUN_ROW_CLASS,
        "w-full text-left transition-colors duration-base hover:bg-surface-2 focus-visible:shadow-focus-inset focus-visible:outline-none"
      )}
      onClick={openReport}
      type="button"
    >
      <span className="min-w-0">
        <span className={HOME_RUN_TITLE_CLASS}>{finding.title}</span>
        <span className={HOME_RUN_SUBTITLE_CLASS}>
          <span className="truncate">
            {finding.scope} · {finding.when}
          </span>
        </span>
      </span>
      {finding.waiting ? (
        <Pill size="sm" tone="accent">
          Your call
        </Pill>
      ) : null}
    </button>
  );
}

export function FindingsPanel({ data }: { data: ProfileData }) {
  if (data.findings.length === 0) return null;
  return (
    <Section count={data.findings.length} label="What it found">
      <Panel bodyClassName="p-0" className="overflow-hidden">
        <div className="divide-y divide-line-soft">
          {data.findings.map(finding => (
            <FindingRow finding={finding} key={finding.id} />
          ))}
        </div>
      </Panel>
    </Section>
  );
}

function WorkEvent({ item }: { item: WorkItem }) {
  return (
    <TimelineEvent
      description={item.detail}
      icon={item.tone === "success" ? Check : undefined}
      time={item.when}
      title={item.title}
      tone={item.tone ?? "neutral"}
    />
  );
}

export function RecentWork({ data }: { data: ProfileData }) {
  if (data.work.length === 0) return null;
  return (
    <Section label="Recent work">
      <Panel>
        <Timeline ariaLabel="Recent work" className="gap-3">
          {data.work.map(item => (
            <WorkEvent item={item} key={item.id} />
          ))}
        </Timeline>
      </Panel>
    </Section>
  );
}
