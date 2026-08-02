import { Link } from "@tanstack/react-router";
import { Repeat2 } from "lucide-react";

import { ListingRow, Pill } from "@compozy/ui";

import {
  hasHumanGate,
  isUnboundedCap,
  iterationCapLabel,
  loopCategory,
  loopInputCount,
  loopSourceLabel,
  successRateLabel,
} from "../../lib/loop-catalog";
import { loopRunBestLabel } from "../../lib/loop-generation-presentation";
import type { LoopCatalogEntry } from "../../types";
import { LoopStatusPill } from "../loop-status-pill";
import { LoopRunButton } from "./loop-run-button";

interface LoopCatalogRowProps {
  entry: LoopCatalogEntry;
  onRun: (entry: LoopCatalogEntry) => void;
}

export function LoopCatalogRow({ entry, onRun }: LoopCatalogRowProps) {
  const category = loopCategory(entry);
  const inputCount = loopInputCount(entry);
  const unbounded = isUnboundedCap(entry);
  const sourceLabel = loopSourceLabel(entry);
  const best = entry.last_run ? loopRunBestLabel(entry.last_run) : null;
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
            <Pill size="xs" tone="neutral">
              {sourceLabel}
            </Pill>
            {unbounded ? (
              <Pill size="xs" tone="neutral">
                ∞ cap
              </Pill>
            ) : null}
            <ListingRow.Slug>v{entry.version}</ListingRow.Slug>
          </ListingRow.Name>
          {entry.contract.goal ? (
            <ListingRow.Description>{entry.contract.goal}</ListingRow.Description>
          ) : null}
          <ListingRow.Meta>
            <span className="font-mono text-mono-id text-subtle">{inputCount} inputs</span>
            <ListingRow.MetaDot />
            <span>iteration cap {iterationCapLabel(entry.contract.iteration_cap)}</span>
            {hasHumanGate(entry) ? (
              <>
                <ListingRow.MetaDot />
                <span>human gate</span>
              </>
            ) : null}
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail className="col-span-2 justify-between gap-3 sm:col-auto sm:justify-self-auto">
        {category ? (
          <Pill className="hidden lg:inline-flex" mono size="sm" tone="neutral">
            {category}
          </Pill>
        ) : null}
        {entry.last_run ? (
          <LoopStatusPill className="hidden sm:inline-flex" status={entry.last_run.status} />
        ) : null}
        {best ? (
          <Pill className="shrink-0" data-testid="loop-catalog-best" mono size="xs" tone="success">
            best {best}
          </Pill>
        ) : null}
        <ListingRow.Stat className="hidden w-20 xl:flex">
          <ListingRow.Stat.Value>{successRateLabel(entry.success_rate_30d)}</ListingRow.Stat.Value>
          <ListingRow.Stat.Label>{entry.aggregate_30d.runs} runs · 30d</ListingRow.Stat.Label>
        </ListingRow.Stat>
        <LoopRunButton loopName={entry.name} onRun={() => onRun(entry)} />
      </ListingRow.Trail>
    </ListingRow>
  );
}
