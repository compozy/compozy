import { TableHead, TableHeader, TableRow } from "@compozy/ui";

/** The sessions table's header, shared by the live table and its skeleton. */
export function AgentSessionsTableHead() {
  return (
    <TableHeader>
      <TableRow>
        <TableHead className="w-2/5">Session</TableHead>
        <TableHead>Status</TableHead>
        <TableHead className="text-right">Duration</TableHead>
        <TableHead className="text-right">Last activity</TableHead>
        <TableHead className="w-10">
          <span className="sr-only">Actions</span>
        </TableHead>
      </TableRow>
    </TableHeader>
  );
}
