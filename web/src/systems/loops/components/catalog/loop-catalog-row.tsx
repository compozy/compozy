import { Link } from "@tanstack/react-router";
import { Repeat2 } from "lucide-react";

import { ListingRow } from "@compozy/ui";

import { loopMonthActivity } from "../../lib/loop-catalog";
import type { LoopCatalogEntry } from "../../types";
import { LoopStatusMark } from "../loop-status-mark";
import { LoopCatalogFacts } from "./loop-catalog-facts";
import { LoopRunButton } from "./loop-run-button";

interface LoopCatalogRowProps {
  entry: LoopCatalogEntry;
  onRun: (entry: LoopCatalogEntry) => void;
}

export function LoopCatalogRow({ entry, onRun }: LoopCatalogRowProps) {
  const activity = loopMonthActivity(entry.aggregate_30d.runs, entry.success_rate_30d);
  return (
    <ListingRow
      className="max-sm:grid-cols-[var(--size-icon-well-row)_minmax(0,1fr)]"
      data-testid="loop-catalog-row"
      data-loop={entry.name}
    >
      <ListingRow.Link
        render={
          <Link to="/loops/$name" params={{ name: entry.name }} aria-label={`Open ${entry.name}`} />
        }
      >
        <ListingRow.Icon>
          <Repeat2 aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>{entry.name}</ListingRow.Title>
            <ListingRow.Slug>v{entry.version}</ListingRow.Slug>
          </ListingRow.Name>
          {entry.contract.goal ? (
            <ListingRow.Description>{entry.contract.goal}</ListingRow.Description>
          ) : null}
          <ListingRow.Meta>
            <LoopCatalogFacts entry={entry} separator="dot" />
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail className="col-span-2 justify-between gap-3 sm:col-auto sm:justify-self-auto">
        {entry.last_run ? <LoopStatusMark status={entry.last_run.status} /> : null}
        {/* max-content keeps "N runs this month" on one line beside its rate. */}
        <ListingRow.Stat
          className="hidden min-w-max xl:flex"
          data-testid="loop-catalog-row-activity"
        >
          <ListingRow.Stat.Value>{activity.rate}</ListingRow.Stat.Value>
          <ListingRow.Stat.Label>{activity.runs}</ListingRow.Stat.Label>
        </ListingRow.Stat>
        <LoopRunButton loopName={entry.name} onRun={() => onRun(entry)} />
      </ListingRow.Trail>
    </ListingRow>
  );
}
