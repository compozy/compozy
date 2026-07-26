import type { ReactNode } from "react";
import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { Eyebrow, cn } from "@agh/ui";

interface FeatureCardProps {
  eyebrow?: string;
  title: ReactNode;
  description: ReactNode;
  icon?: ReactNode;
  /** Optional doc path that backs this claim. Renders as a subtle "source" link. */
  cite?: { href: string; label?: string };
  className?: string;
}

export function FeatureCard({
  eyebrow,
  title,
  description,
  icon,
  cite,
  className,
}: FeatureCardProps) {
  return (
    <article
      className={cn(
        "flex flex-col gap-3 rounded-diagram border border-line bg-canvas-soft p-6 transition-colors hover:border-accent/40",
        className
      )}
    >
      {icon ? (
        <div className="flex size-10 items-center justify-center rounded-icon-well bg-elevated text-accent">
          {icon}
        </div>
      ) : null}
      {eyebrow ? <Eyebrow className="text-accent">{eyebrow}</Eyebrow> : null}
      <h3 className="text-base font-medium leading-snug text-fg">{title}</h3>
      <p className="text-sm leading-relaxed text-muted">{description}</p>
      {cite ? (
        <Link
          href={cite.href}
          className="eyebrow mt-auto inline-flex items-center gap-1 pt-2 text-subtle transition-colors hover:text-accent"
        >
          {cite.label ?? "source"}
          <ArrowUpRight aria-hidden className="size-3" />
        </Link>
      ) : null}
    </article>
  );
}
