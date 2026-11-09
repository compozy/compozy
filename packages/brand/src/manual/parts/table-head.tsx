// Header row for the manual's spec tables: mono uppercase labels on a ruled line.
import { cn } from "@compozy/ui/utils";

/** Column labels, in order; each label is unique within its table. */
export function TableHead({
  columns,
  className,
}: {
  columns: readonly string[];
  className?: string;
}) {
  return (
    <thead>
      <tr
        className={cn(
          "border-b border-(--b-line) text-left mono-label text-[11.5px] tracking-[0.08em] text-(--b-muted)",
          className
        )}
      >
        {columns.map(column => (
          <th key={column} className="px-5 py-3 font-medium">
            {column}
          </th>
        ))}
      </tr>
    </thead>
  );
}
