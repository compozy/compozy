import { Link } from "@tanstack/react-router";
import { Repeat2 } from "@compozy/ui/icons";

import { ListingRow } from "@compozy/ui";

import { WorkspaceBadge, type WorkspaceBadgeIdentity } from "@/systems/workspace";

import { loopMonthActivity } from "../../lib/loop-catalog";
import { loopFactsPresent } from "../../lib/loop-catalog-presentation";
import type { LoopCatalogEntry } from "../../types";
import { LoopStatusMark } from "../loop-status-mark";
import { LoopCatalogFacts } from "./loop-catalog-facts";
import { LoopRunButton } from "./loop-run-button";

const UNBOUND_SEARCH: { workspace?: string } = {};

interface LoopCatalogRowProps {
  entry: LoopCatalogEntry;
  onRun: (entry: LoopCatalogEntry) => void;
  /** Keeps the row's link on its owner's binding. */
  linkSearch?: { workspace?: string };
  /** Under All projects: the row's project, leading the meta line. */
  project?: WorkspaceBadgeIdentity;
}

export function LoopCatalogRow({
  entry,
  onRun,
  linkSearch = UNBOUND_SEARCH,
  project,
}: LoopCatalogRowProps) {
  const activity = loopMonthActivity(entry.aggregate_30d.runs, entry.success_rate_30d);
  return (
    <ListingRow
      className="max-sm:grid-cols-[var(--size-icon-well-row)_minmax(0,1fr)]"
      data-testid="loop-catalog-row"
      data-loop={entry.name}
    >
      <ListingRow.Link
        render={
          <Link
            aria-label={`Open ${entry.name}`}
            params={{ name: entry.name }}
            search={linkSearch}
            to="/loops/$name"
          />
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
            {project ? (
              <>
                <WorkspaceBadge data-testid="loop-catalog-row-project" identity={project} />
                {loopFactsPresent(entry) ? <ListingRow.MetaDot /> : null}
              </>
            ) : null}
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
