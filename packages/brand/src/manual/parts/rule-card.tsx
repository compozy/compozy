// Rule lists: a labelled card of bullet rules, and a list of rules led by a bold term.
import { cn } from "@compozy/ui/utils";
import type { ReactNode } from "react";

import { card, lab, rules } from "../cls";

/** A card with a mono label over a bullet list; children are the <li> rules. */
export function RuleCard({
  title,
  className,
  children,
}: {
  title: ReactNode;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn(card, className)}>
      <p className={lab}>{title}</p>
      <ul className={cn(rules, "mt-4 text-[15px]")}>{children}</ul>
    </div>
  );
}

/** Rules keyed by their bold lead term; terms are unique within one list. */
export type TermRuleItems = readonly (readonly [term: string, body: ReactNode])[];

/** Bullet rules that each open with a bold term, e.g. "**Zero is a value**, missing is hatched." */
export function TermRules({ items, className }: { items: TermRuleItems; className?: string }) {
  return (
    <ul className={cn(rules, className)}>
      {items.map(([term, body]) => (
        <li key={term}>
          <strong>{term}</strong>
          {body}
        </li>
      ))}
    </ul>
  );
}
