// Rule rows used across the editorial chapter: label left, rule right, band-aware.
import { cn } from "@compozy/ui/utils";

export function Rows({ rows, className }: { rows: [string, string][]; className?: string }) {
  return (
    <dl className={cn("m-0 grid gap-0", className)}>
      {rows.map(([t, d]) => (
        <div
          key={t}
          className="grid gap-1 border-b border-(--b-line) py-3 last:border-0 md:grid-cols-[140px_minmax(0,1fr)] md:gap-6"
        >
          <dt className="text-[14.5px] font-semibold">{t}</dt>
          <dd className="m-0 text-[14.5px] text-(--b-muted)">{d}</dd>
        </div>
      ))}
    </dl>
  );
}
