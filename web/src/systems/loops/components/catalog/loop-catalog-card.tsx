import { Link } from "@tanstack/react-router";
import { Repeat2 } from "lucide-react";

import { CatalogCard } from "@compozy/ui";

import { loopCategory, loopSourceLabel, successRateLabel } from "../../lib/loop-catalog";
import { loopFactsSegments } from "../../lib/loop-catalog-presentation";
import type { LoopCatalogEntry } from "../../types";
import { LoopStatusPill } from "../loop-status-pill";
import { LoopRunButton } from "./loop-run-button";

interface LoopCatalogCardProps {
  entry: LoopCatalogEntry;
  onRun: (entry: LoopCatalogEntry) => void;
}

export function LoopCatalogCard({ entry, onRun }: LoopCatalogCardProps) {
  const category = loopCategory(entry);
  const sourceLabel = loopSourceLabel(entry);
  return (
    <CatalogCard actionable data-loop={entry.name} data-testid={`loop-catalog-card-${entry.name}`}>
      <Link
        aria-label={`Open ${entry.name}`}
        className="flex min-w-0 flex-col gap-3"
        params={{ name: entry.name }}
        to="/loops/$name"
      >
        <div className="flex items-start gap-3">
          <CatalogCard.Logo>
            <Repeat2 aria-hidden="true" className="size-4" />
          </CatalogCard.Logo>
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <CatalogCard.Title>{entry.name}</CatalogCard.Title>
            <CatalogCard.Meta>
              <span className="font-mono">{`v${entry.version}`}</span>
              <span>{sourceLabel}</span>
              {category ? <span>{category}</span> : null}
            </CatalogCard.Meta>
          </div>
        </div>
        {entry.contract.goal ? (
          <CatalogCard.Description>{entry.contract.goal}</CatalogCard.Description>
        ) : null}
        <p className="font-mono text-mono-id text-subtle">{loopFactsSegments(entry).join(" · ")}</p>
      </Link>
      <CatalogCard.Actions className={entry.last_run ? "justify-between gap-3" : "justify-end"}>
        {entry.last_run ? <LoopStatusPill status={entry.last_run.status} /> : null}
        <div className="flex min-w-0 items-center gap-3">
          <span
            className="flex min-w-0 flex-col items-end"
            data-testid={`loop-catalog-card-stat-${entry.name}`}
          >
            <span className="font-mono text-small-body tabular-nums text-fg">
              {successRateLabel(entry.success_rate_30d)}
            </span>
            <span className="text-micro text-faint">{entry.aggregate_30d.runs} runs · 30d</span>
          </span>
          <LoopRunButton loopName={entry.name} onRun={() => onRun(entry)} />
        </div>
      </CatalogCard.Actions>
    </CatalogCard>
  );
}
