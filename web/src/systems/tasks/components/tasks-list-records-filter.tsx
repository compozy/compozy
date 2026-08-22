import { GitBranch } from "lucide-react";

import { PillGroup, type PillGroupItem } from "@compozy/ui";

import type { TaskRecordsFilter } from "../types";

export interface TasksListRecordsFilterProps {
  value: TaskRecordsFilter;
  onChange: (next: TaskRecordsFilter) => void;
}

/**
 * Both states are named rather than one button carrying a pressed state, so the
 * current population is always legible without decoding a toggle.
 */
const RECORDS_FILTER_ITEMS: ReadonlyArray<PillGroupItem<TaskRecordsFilter>> = [
  { value: "work", label: "Work items", testId: "tasks-records-filter-work" },
  {
    value: "loop",
    label: (
      <>
        <GitBranch aria-hidden="true" />+ loop records
      </>
    ),
    testId: "tasks-records-filter-loop",
  },
];

/**
 * The quiet reveal control (US-002). Loop execution records leave the listing by
 * default (ADR-001); this puts them back for the current context only — it
 * resets when the surface changes and is deliberately not a persisted setting.
 */
export function TasksListRecordsFilter({ value, onChange }: TasksListRecordsFilterProps) {
  return (
    <PillGroup
      aria-label="Records shown"
      data-testid="tasks-records-filter"
      items={RECORDS_FILTER_ITEMS}
      onChange={onChange}
      size="sm"
      value={value}
    />
  );
}
