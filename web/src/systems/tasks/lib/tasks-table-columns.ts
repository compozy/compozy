/**
 * Column visibility for the Tasks table. The table sits in an `@container`, so
 * columns drop by pane width instead of viewport width: a half-width tile keeps
 * Task · Status · Owner (the title keeps most of the row), wider panes add the
 * identifier (768px), priority (896px) and recency (1024px).
 */
export const TASKS_TABLE_COLUMN_CLASS = {
  id: "hidden w-28 @3xl:table-cell",
  status: "w-36",
  priority: "hidden w-24 @4xl:table-cell",
  owner: "w-14",
  updated: "hidden w-20 @5xl:table-cell",
} as const;
