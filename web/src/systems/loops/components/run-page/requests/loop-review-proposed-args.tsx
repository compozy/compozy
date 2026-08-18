import { Minus, Pencil } from "lucide-react";

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@compozy/ui";

export interface LoopReviewProposedArgsProps {
  proposed: unknown;
  edited?: Readonly<Record<string, string>>;
}

interface ProposedArgRow {
  name: string;
  original: string | null;
  edited: string | null;
  change: "carried" | "changed" | "removed" | "added";
}

export function LoopReviewProposedArgs({ proposed, edited }: LoopReviewProposedArgsProps) {
  const rows = proposedArgRows(proposed, edited);
  if (rows.length === 0) return null;
  const isEditing = edited !== undefined;
  return (
    <div
      className="overflow-hidden rounded-md border border-line-soft"
      data-testid="loop-review-proposed-args"
    >
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="h-8 px-3">Argument</TableHead>
            <TableHead className="h-8 px-3">{isEditing ? "Original" : "Value"}</TableHead>
            {isEditing ? <TableHead className="h-8 px-3">Edited</TableHead> : null}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map(row => (
            <TableRow className="border-line-soft hover:bg-transparent" key={row.name}>
              <TableCell className="px-3 py-2 align-top font-mono text-mono-id whitespace-nowrap text-muted">
                {row.name}
              </TableCell>
              <TableCell className="px-3 py-2 align-top font-mono text-mono-id break-all text-fg">
                {row.original ?? <ArgTag change="added" />}
              </TableCell>
              {isEditing ? (
                <TableCell className="px-3 py-2 align-top font-mono text-mono-id break-all text-fg">
                  <EditedCell row={row} />
                </TableCell>
              ) : null}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function EditedCell({ row }: { row: ProposedArgRow }) {
  if (row.change === "removed") return <ArgTag change="removed" />;
  if (row.change === "carried") return <span className="text-muted">{row.edited}</span>;
  return (
    <span className="flex flex-wrap items-center gap-1.5">
      <span className="text-fg-strong">{row.edited}</span>
      <ArgTag change={row.change} />
    </span>
  );
}

function ArgTag({ change }: { change: ProposedArgRow["change"] }) {
  if (change === "removed") {
    return (
      <span className="inline-flex items-center gap-1 font-sans text-form-hint font-medium text-danger">
        <Minus aria-hidden="true" className="size-2.5" />
        removed
      </span>
    );
  }
  if (change === "added") {
    return (
      <span className="inline-flex items-center gap-1 font-sans text-form-hint font-medium text-accent">
        <Pencil aria-hidden="true" className="size-2.5" />
        added
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1 font-sans text-form-hint font-medium text-accent">
      <Pencil aria-hidden="true" className="size-2.5" />
      changed
    </span>
  );
}

function proposedArgRows(
  proposed: unknown,
  edited?: Readonly<Record<string, string>>
): ProposedArgRow[] {
  const original = asRecord(proposed);
  const names = new Set<string>(original ? Object.keys(original) : []);
  if (edited) for (const name of Object.keys(edited)) names.add(name);
  return [...names].map(name => {
    const originalValue = original && name in original ? printableValue(original[name]) : null;
    if (!edited) return { name, original: originalValue, edited: null, change: "carried" };
    const editedValue = (edited[name] ?? "").trim();
    if (editedValue === "") {
      return {
        name,
        original: originalValue,
        edited: null,
        change: originalValue === null ? "carried" : "removed",
      };
    }
    if (originalValue === null)
      return { name, original: null, edited: editedValue, change: "added" };
    return {
      name,
      original: originalValue,
      edited: editedValue,
      change: editedValue === originalValue ? "carried" : "changed",
    };
  });
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function printableValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (value === undefined) return "";
  try {
    return JSON.stringify(value) ?? String(value);
  } catch {
    return String(value);
  }
}
