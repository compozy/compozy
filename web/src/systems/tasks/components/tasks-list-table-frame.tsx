import type { ReactNode } from "react";

import { Table, TableHead, TableHeader, TableRow } from "@compozy/ui";
import { cn } from "@/lib/utils";

import { TASKS_TABLE_COLUMN_CLASS } from "../lib/tasks-table-columns";

/**
 * The table and its header, shared by the loaded list and its skeleton so the header renders
 * at once with real labels. `total` is the whole list's size; absent while it is not known yet.
 */
export function TasksListTableFrame({ total, children }: { total?: number; children: ReactNode }) {
  return (
    <Table className="table-fixed" data-testid="tasks-list-table" overflowX="hidden">
      <TableHeader>
        <TableRow>
          <TableHead className="pl-4">
            Task
            {total === undefined ? null : (
              <>
                {" "}
                <span className="font-normal text-subtle tabular-nums" data-slot="tasks-list-total">
                  ({total})
                </span>
              </>
            )}
          </TableHead>
          <TableHead className={TASKS_TABLE_COLUMN_CLASS.id}>ID</TableHead>
          <TableHead className={TASKS_TABLE_COLUMN_CLASS.status}>Status</TableHead>
          <TableHead className={TASKS_TABLE_COLUMN_CLASS.priority}>Priority</TableHead>
          <TableHead className={cn("pr-4 @5xl:pr-3", TASKS_TABLE_COLUMN_CLASS.owner)}>
            <span className="sr-only">Owner</span>
          </TableHead>
          <TableHead className={cn("pr-4", TASKS_TABLE_COLUMN_CLASS.updated)}>Updated</TableHead>
        </TableRow>
      </TableHeader>
      {children}
    </Table>
  );
}
