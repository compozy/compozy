import { Link } from "@tanstack/react-router";
import { Repeat2 } from "lucide-react";

import { ListingRow, Pill } from "@agh/ui";

import {
  hasHumanGate,
  isUnboundedCap,
  iterationCapLabel,
  loopCategory,
  loopInputCount,
  loopSourceLabel,
  successRateLabel,
} from "../../lib/loop-catalog";
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
  return (
    <ListingRow data-testid="loop-catalog-row" data-loop={entry.name}>
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
      <ListingRow.Trail className="gap-3">
        {category ? (
          <Pill className="hidden sm:inline-flex" mono size="sm" tone="neutral">
            {category}
          </Pill>
        ) : null}
        {entry.last_run ? <LoopStatusPill status={entry.last_run.status} /> : null}
        <ListingRow.Stat className="w-20">
          <ListingRow.Stat.Value>{successRateLabel(entry.success_rate_30d)}</ListingRow.Stat.Value>
          <ListingRow.Stat.Label>{entry.aggregate_30d.runs} runs · 30d</ListingRow.Stat.Label>
        </ListingRow.Stat>
        <LoopRunButton loopName={entry.name} onRun={() => onRun(entry)} />
      </ListingRow.Trail>
    </ListingRow>
  );
}
