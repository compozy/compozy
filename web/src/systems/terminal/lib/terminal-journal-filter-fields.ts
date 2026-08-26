import type { Filter, FilterFieldsConfig } from "@compozy/ui";

import type { TerminalActorKind, TerminalJournalFilters } from "../types";

/**
 * The journal's filters, spoken through the shared `Filters` primitive.
 *
 * Chips are the single source of interaction state; the query reads the
 * `TerminalJournalFilters` projection derived from them. Only complete values
 * project — a text chip still being typed filters nothing yet.
 */

const IS_OPERATOR = [{ value: "is", label: "is" }];

const ACTOR_KINDS: readonly TerminalActorKind[] = ["human", "agent", "system"];

function isActorKind(value: string): value is TerminalActorKind {
  return (ACTOR_KINDS as readonly string[]).includes(value);
}

export function terminalJournalFilterFields(): FilterFieldsConfig<string> {
  return [
    {
      key: "actor",
      label: "Who",
      type: "select",
      operators: IS_OPERATOR,
      options: [
        { value: "human", label: "A person" },
        { value: "agent", label: "An agent" },
        { value: "system", label: "CompozyOS" },
      ],
    },
    {
      key: "result",
      label: "Result",
      type: "select",
      operators: IS_OPERATOR,
      options: [{ value: "failed", label: "Finished with errors" }],
    },
    {
      key: "since",
      label: "Since",
      type: "text",
      operators: IS_OPERATOR,
      placeholder: "24h",
    },
    {
      key: "terminal",
      label: "Terminal",
      type: "text",
      operators: IS_OPERATOR,
      placeholder: "term-…",
    },
  ];
}

/** One chip per fact — the journal never carries two values for one field. */
export function terminalJournalChipsFromFilters(filters: TerminalJournalFilters): Filter<string>[] {
  return [
    ...(filters.actor
      ? [{ id: "actor", field: "actor", operator: "is", values: [filters.actor] }]
      : []),
    ...(filters.failed
      ? [{ id: "result", field: "result", operator: "is", values: ["failed"] }]
      : []),
    ...(filters.since
      ? [{ id: "since", field: "since", operator: "is", values: [filters.since] }]
      : []),
    ...(filters.terminalId
      ? [{ id: "terminal", field: "terminal", operator: "is", values: [filters.terminalId] }]
      : []),
  ];
}

export function terminalJournalFiltersFromChips(chips: Filter<string>[]): TerminalJournalFilters {
  const next: TerminalJournalFilters = {};
  for (const chip of chips) {
    const value = chip.values[0]?.trim() ?? "";
    if (value === "") continue;
    if (chip.field === "actor" && isActorKind(value)) next.actor = value;
    if (chip.field === "result" && value === "failed") next.failed = true;
    if (chip.field === "since") next.since = value;
    if (chip.field === "terminal") next.terminalId = value;
  }
  return next;
}
