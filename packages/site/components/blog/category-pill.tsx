import { cn } from "@agh/ui";
import Link from "next/link";

export interface CategoryPillProps {
  label: string;
  count?: number;
  href: string;
  active?: boolean;
}

export function CategoryPill({ label, count, href, active = false }: CategoryPillProps) {
  const accessibleLabel = count === undefined ? label : `${label} (${count})`;

  return (
    <Link
      href={href}
      aria-current={active ? "page" : undefined}
      aria-label={accessibleLabel}
      className={cn(
        "inline-flex h-8 items-center gap-2 rounded-full border px-3.5 font-sans text-small-body font-medium transition-colors",
        active ? "border-accent bg-elevated text-fg" : "border-line text-muted hover:text-fg"
      )}
    >
      <span>{label}</span>
      {count !== undefined && (
        <span className="font-mono text-badge tracking-mono text-muted">
          {String(count).padStart(2, "0")}
        </span>
      )}
    </Link>
  );
}
