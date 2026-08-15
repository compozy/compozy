import { Fragment, type ReactNode } from "react";

import { ListingRow, MonoId, Time } from "@compozy/ui";

import { loopFactsSegments, loopLastRunFact } from "../../lib/loop-catalog-presentation";
import type { LoopCatalogEntry } from "../../types";

interface LoopCatalogFactsProps {
  entry: LoopCatalogEntry;
  /** Rows use the listing meta dot; cards join with a middot. */
  separator: "dot" | "middot";
}

/**
 * One facts list for both catalog views. Sans/faint for declared facts; mono
 * only on last-run machine identity (`MonoId` + `Time`).
 */
export function LoopCatalogFacts({ entry, separator }: LoopCatalogFactsProps) {
  const texts = loopFactsSegments(entry);
  const lastRun = loopLastRunFact(entry);
  const nodes: { key: string; node: ReactNode }[] = texts.map(text => ({
    key: text,
    node: <span>{text}</span>,
  }));
  if (lastRun) {
    nodes.push({
      key: lastRun.id,
      node: (
        <span className="inline-flex items-center gap-1 font-mono text-mono-id">
          <MonoId value={lastRun.id} />
          <span aria-hidden="true">·</span>
          <Time iso={lastRun.iso} />
        </span>
      ),
    });
  }
  return (
    <>
      {nodes.map((item, index) => (
        <Fragment key={item.key}>
          {index > 0 ? (
            separator === "dot" ? (
              <ListingRow.MetaDot />
            ) : (
              <span aria-hidden="true"> · </span>
            )
          ) : null}
          {item.node}
        </Fragment>
      ))}
    </>
  );
}
