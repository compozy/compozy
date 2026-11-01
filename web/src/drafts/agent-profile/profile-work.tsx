/** What the agent has actually done: findings first, then the running log. */
import { protoHref } from "@compozy/proto";
import { Check, Search } from "lucide-react";

import { ListingRow, Panel, Pill, Timeline, TimelineEvent } from "@compozy/ui";

import type { Finding, ProfileData, WorkItem } from "./profile-fixtures";

function openReport() {
  window.location.assign(protoHref("stays-on-reports", "proposal"));
}

function FindingRow({ finding }: { finding: Finding }) {
  return (
    <ListingRow>
      <ListingRow.Link
        className="text-left"
        render={<button aria-label={`Open: ${finding.title}`} onClick={openReport} type="button" />}
      >
        <ListingRow.Icon>
          <Search aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title className="whitespace-normal">{finding.title}</ListingRow.Title>
          </ListingRow.Name>
          <ListingRow.Meta>
            <span className="truncate">
              {finding.scope} · {finding.when}
            </span>
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      {finding.waiting ? (
        <ListingRow.Trail>
          <Pill size="sm" tone="accent">
            Your call
          </Pill>
        </ListingRow.Trail>
      ) : null}
    </ListingRow>
  );
}

export function FindingsPanel({ data }: { data: ProfileData }) {
  if (data.findings.length === 0) return null;
  return (
    <Panel bodyClassName="p-0" meta={data.findings.length} title="What it found">
      <div className="flex flex-col divide-y divide-line-soft">
        {data.findings.map(finding => (
          <FindingRow finding={finding} key={finding.id} />
        ))}
      </div>
    </Panel>
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
    <Panel title="Recent work">
      <Timeline ariaLabel="Recent work" className="gap-3">
        {data.work.map(item => (
          <WorkEvent item={item} key={item.id} />
        ))}
      </Timeline>
    </Panel>
  );
}
