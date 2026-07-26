import { Eyebrow } from "@agh/ui";
import Link from "next/link";

export interface BlogEmptyStateAction {
  href: string;
  label: string;
}

export interface BlogEmptyStateProps {
  eyebrow: string;
  title: string;
  description: string;
  primaryAction: BlogEmptyStateAction;
  secondaryAction?: BlogEmptyStateAction;
}

export function BlogEmptyState({
  eyebrow,
  title,
  description,
  primaryAction,
  secondaryAction,
}: BlogEmptyStateProps) {
  return (
    <section className="rounded-xl border border-line bg-canvas-soft p-6">
      <Eyebrow className="text-accent">{eyebrow}</Eyebrow>
      <h2 className="mt-4 max-w-[26ch] font-sans text-site-card-title font-semibold leading-tight tracking-tight text-fg">
        {title}
      </h2>
      <p className="mt-4 max-w-[58ch] text-sm leading-7 text-muted">{description}</p>
      <div className="mt-6 flex flex-wrap gap-3">
        <Link
          href={primaryAction.href}
          className="inline-flex h-9 items-center justify-center rounded-lg border border-line px-3.5 font-sans text-sm font-medium text-fg transition-colors hover:bg-hover"
        >
          {primaryAction.label}
        </Link>
        {secondaryAction && (
          <Link
            href={secondaryAction.href}
            className="inline-flex h-9 items-center justify-center rounded-lg border border-line px-3.5 font-sans text-sm font-medium text-fg transition-colors hover:bg-hover"
          >
            {secondaryAction.label}
          </Link>
        )}
      </div>
    </section>
  );
}
