import { Link } from "@tanstack/react-router";
import { Repeat2 } from "lucide-react";

import { CatalogCard } from "@compozy/ui";

import type { LoopCatalogEntry } from "../../types";
import { LoopStatusMark } from "../loop-status-mark";
import { LoopCatalogFacts } from "./loop-catalog-facts";
import { LoopRunButton } from "./loop-run-button";

interface LoopCatalogCardProps {
  entry: LoopCatalogEntry;
  onRun: (entry: LoopCatalogEntry) => void;
}

export function LoopCatalogCard({ entry, onRun }: LoopCatalogCardProps) {
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
            </CatalogCard.Meta>
          </div>
        </div>
        {entry.contract.goal ? (
          <CatalogCard.Description className="line-clamp-2">
            {entry.contract.goal}
          </CatalogCard.Description>
        ) : null}
        <p className="flex flex-wrap items-center text-eyebrow text-faint">
          <LoopCatalogFacts entry={entry} separator="middot" />
        </p>
      </Link>
      <CatalogCard.Actions className={entry.last_run ? "justify-between gap-3" : "justify-end"}>
        {entry.last_run ? <LoopStatusMark status={entry.last_run.status} /> : null}
        <LoopRunButton loopName={entry.name} onRun={() => onRun(entry)} />
      </CatalogCard.Actions>
    </CatalogCard>
  );
}
