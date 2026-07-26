import { Link } from "@tanstack/react-router";
import { Repeat2 } from "lucide-react";

import { CatalogCard, Pill } from "@agh/ui";

import { loopCategory, loopSourceLabel } from "../../lib/loop-catalog";
import type { LoopCatalogEntry } from "../../types";
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
            </CatalogCard.Meta>
          </div>
        </div>
        {entry.contract.goal ? (
          <CatalogCard.Description>{entry.contract.goal}</CatalogCard.Description>
        ) : null}
      </Link>
      <CatalogCard.Actions className={category ? "justify-between" : "justify-end"}>
        {category ? (
          <Pill mono size="sm" tone="neutral">
            {category}
          </Pill>
        ) : null}
        <LoopRunButton loopName={entry.name} onRun={() => onRun(entry)} />
      </CatalogCard.Actions>
    </CatalogCard>
  );
}
