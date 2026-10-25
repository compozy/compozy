import { TableHead, TableHeader, TableRow } from "@compozy/ui";

/**
 * The journal's column heads, shared by the live table and its loading shape. `compact` folds the
 * attribute columns away while the record rail is open.
 */
export function TerminalJournalColumns({ compact = false }: { compact?: boolean }) {
  return (
    <TableHeader>
      <TableRow>
        <TableHead className="w-18">When</TableHead>
        <TableHead className="w-35">Who</TableHead>
        <TableHead>Command</TableHead>
        <TableHead className="w-43">Result</TableHead>
        {compact ? null : (
          <>
            <TableHead className="w-30">Permission</TableHead>
            <TableHead className="w-29">Confidence</TableHead>
            <TableHead aria-label="Recording" className="w-13" />
          </>
        )}
      </TableRow>
    </TableHeader>
  );
}
