/** Every session action against every menu host: what each has today and after the change. */
import {
  Eyebrow,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@compozy/ui";

import { cn } from "@/lib/utils";

/** `has` today and after · `new` added here · `fixed` was partial, now everywhere · `na` cannot apply. */
type Cell = "has" | "new" | "fixed" | { na: string };

interface Row {
  group: string;
  action: string;
  cells: [Cell, Cell, Cell, Cell];
}

const ONE = { na: "One session only" };

const ROWS: Row[] = [
  { group: "Top level", action: "Rename session", cells: ["has", "has", ONE, ONE] },
  { group: "Copy ▸", action: "Session ID(s) · Title(s)", cells: ["has", "has", "has", "has"] },
  { group: "Copy ▸", action: "Branch name(s)", cells: ["new", "new", "new", "new"] },
  { group: "Copy ▸", action: "Pull request link", cells: ["new", "new", ONE, ONE] },
  {
    group: "New from this ▸",
    action: "Continue with another agent… · Fork session…",
    cells: ["has", "has", { na: "One conversation each" }, { na: "One conversation each" }],
  },
  {
    group: "Pull request ▸",
    action: "Fix checks · Address review · Open PR · Refresh",
    cells: ["new", "new", ONE, ONE],
  },
  {
    group: "Move ▸",
    action: "Folder · Remove from folder",
    cells: ["fixed", "new", "has", "has"],
  },
  { group: "Move ▸", action: "Nest under…", cells: ["has", "has", "new", "new"] },
  { group: "Move ▸", action: "Make top-level", cells: ["has", "has", "has", "has"] },
  { group: "Move ▸", action: "Move to profile", cells: ["has", "has", "new", "new"] },
  {
    group: "Move ▸",
    action: "Move up · Move down",
    cells: [
      "fixed",
      { na: "Columns order the board" },
      { na: "Drag the rows" },
      { na: "Columns order the board" },
    ],
  },
  { group: "Top level", action: "Mark as seen", cells: ["has", "has", "has", "has"] },
  {
    group: "Top level",
    action: "Stop · Move to Later… · Back from Later · Archive · Unarchive",
    cells: ["has", "has", "has", "has"],
  },
  {
    group: "Top level",
    action: "Clear conversation",
    cells: ["has", "has", ONE, ONE],
  },
  { group: "Top level", action: "Delete", cells: ["has", "has", "has", "has"] },
];

const HOSTS = ["List row", "Kanban card", "List selection", "Kanban selection"];

function CellMark({ cell }: { cell: Cell }) {
  if (typeof cell === "object") {
    return <span className="text-small-body text-subtle">— {cell.na}</span>;
  }
  const label = { has: "✓", new: "New", fixed: "Now in both menus" }[cell];
  return (
    <span
      className={cn(
        "text-small-body",
        cell === "has" && "text-muted",
        cell !== "has" && "rounded-sm bg-success-tint px-1.5 py-0.5 font-medium text-success"
      )}
    >
      {label}
    </span>
  );
}

export function ParityMatrix() {
  return (
    <section className="flex max-w-6xl flex-col gap-4">
      <div className="flex flex-col gap-1">
        <Eyebrow>Parity</Eyebrow>
        <p className="max-w-3xl text-body text-muted">
          One menu tree per session and one per selection. Right-click and the{" "}
          <span className="font-mono">…</span> button render the same tree on the list and the
          Kanban; a host drops a verb only when it cannot apply there.{" "}
          <span className="text-fg">Now in both menus</span> marks folder and order verbs that today
          exist on right-click but not behind <span className="font-mono">…</span>.
        </p>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-32">Group</TableHead>
            <TableHead>Action</TableHead>
            {HOSTS.map(host => (
              <TableHead key={host}>{host}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {ROWS.map(row => (
            <TableRow key={`${row.group}-${row.action}`}>
              <TableCell className="text-small-body whitespace-nowrap text-subtle">
                {row.group}
              </TableCell>
              <TableCell className="text-small-body whitespace-normal text-fg">
                {row.action}
              </TableCell>
              {row.cells.map((cell, index) => (
                <TableCell className="w-36 whitespace-normal" key={HOSTS[index]}>
                  <CellMark cell={cell} />
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </section>
  );
}
