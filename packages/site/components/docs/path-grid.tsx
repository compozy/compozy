import { Eyebrow } from "@compozy/ui";
import Link from "next/link";
import type { ReactNode } from "react";
import { DOCS_ICONS } from "@/lib/docs-icons";

interface PathCardProps {
  label: string;
  title: string;
  description: string;
  children: ReactNode;
}

interface PathLinkProps {
  href: string;
  icon: string;
  children: ReactNode;
  /** The one link that names what this audience does first. */
  cta?: boolean;
}

/**
 * The docs landing's audience router: one card per job from the spec's audience model, each opening
 * with the destination that audience reaches for first. Deliberately not the generic
 * icon-heading-text card grid — the value is the link set, so the links carry the card.
 */
export function PathGrid({ children }: { children: ReactNode }) {
  return <div className="not-prose my-8 grid gap-3 md:grid-cols-2">{children}</div>;
}

export function PathCard({ label, title, description, children }: PathCardProps) {
  return (
    <section className="flex flex-col rounded-xl border border-line bg-canvas-soft px-5 pt-[18px] pb-4 transition-colors hover:border-line-strong">
      <Eyebrow className="text-subtle">{label}</Eyebrow>
      <h3 className="mt-2 text-compact-h1 font-semibold tracking-tight text-fg">{title}</h3>
      <p className="mt-1.5 text-sm leading-6 text-muted">{description}</p>
      <div className="mt-3.5 flex flex-col border-t border-line pt-2.5">{children}</div>
    </section>
  );
}

export function PathLink({ href, icon, children, cta = false }: PathLinkProps) {
  const Icon = DOCS_ICONS[icon];
  return (
    <Link
      href={href}
      className={
        cta
          ? "flex h-[30px] items-center gap-2 text-small-body font-medium text-accent transition-colors hover:text-accent-hover"
          : "flex h-[30px] items-center gap-2 text-small-body text-muted transition-colors hover:text-fg"
      }
    >
      {Icon ? <Icon aria-hidden className="size-3 shrink-0 opacity-60" /> : null}
      {children}
    </Link>
  );
}
