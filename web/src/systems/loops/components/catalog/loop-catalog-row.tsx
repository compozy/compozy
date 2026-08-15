import { Link } from "@tanstack/react-router";
import { Repeat2 } from "lucide-react";

import { ListingRow } from "@compozy/ui";

import { successRateLabel } from "../../lib/loop-catalog";
import type { LoopCatalogEntry } from "../../types";
import { LoopStatusPill } from "../loop-status-pill";
import { LoopCatalogFacts } from "./loop-catalog-facts";
import { LoopRunButton } from "./loop-run-button";

interface LoopCatalogRowProps {
  entry: LoopCatalogEntry;
  onRun: (entry: LoopCatalogEntry) => void;
}

export function LoopCatalogRow({ entry, onRun }: LoopCatalogRowProps) {
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
        {entry.last_run ? <LoopStatusPill status={entry.last_run.status} /> : null}
        <ListingRow.Stat className="hidden w-20 xl:flex">
          <ListingRow.Stat.Value>{successRateLabel(entry.success_rate_30d)}</ListingRow.Stat.Value>
          <ListingRow.Stat.Label>{entry.aggregate_30d.runs} runs · 30d</ListingRow.Stat.Label>
        </ListingRow.Stat>
        <LoopRunButton loopName={entry.name} onRun={() => onRun(entry)} />
      </ListingRow.Trail>
    </ListingRow>
  );
}
