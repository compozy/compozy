// Key → value spec rows (a <dl> on the `specs` grid), alone or inside a labelled card.
import { cn } from "@compozy/ui/utils";
import type { ReactNode } from "react";

import { card, lab, specs } from "../cls";

/** Rows keyed by their term; terms are unique within one list. */
export type SpecRows = readonly (readonly [term: string, value: ReactNode])[];

interface SpecListProps {
  rows: SpecRows;
  className?: string;
  /** classes for every <dd>, e.g. mono values */
  valueClassName?: string;
}

export function SpecList({ rows, className, valueClassName }: SpecListProps) {
  return (
    <dl className={cn(specs, "mt-4", className)}>
      {rows.map(([term, value]) => (
        <div key={term}>
          <dt>{term}</dt>
          <dd className={valueClassName}>{value}</dd>
        </div>
      ))}
    </dl>
  );
}

/** A card with a mono label over its spec rows. */
export function SpecCard({
  title,
  className,
  listClassName,
  ...list
}: Omit<SpecListProps, "className"> & {
  title: ReactNode;
  className?: string;
  listClassName?: string;
}) {
  return (
    <div className={cn(card, className)}>
      <p className={lab}>{title}</p>
      <SpecList {...list} className={listClassName} />
    </div>
  );
}
