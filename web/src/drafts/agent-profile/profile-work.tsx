/** What the agent has actually done: findings first, then the running log. */
import { protoHref } from "@compozy/proto";
import { Check } from "@compozy/ui/icons";

import { Panel, Section, Timeline, TimelineEvent } from "@compozy/ui";

import { FindingRow } from "@/systems/agent/components/agent-profile/agent-profile-work";

import type { ProfileData, WorkItem } from "./profile-fixtures";

function openReport() {
  window.location.assign(protoHref("stays-on-reports", "proposal"));
}

export function FindingsPanel({ data }: { data: ProfileData }) {
  if (data.findings.length === 0) return null;
  return (
    <Section count={data.findings.length} label="What it found">
      <Panel bodyClassName="p-0" className="overflow-hidden">
        <div className="divide-y divide-line-soft">
          {data.findings.map(finding => (
            <FindingRow finding={finding} key={finding.id} onOpen={openReport} />
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
