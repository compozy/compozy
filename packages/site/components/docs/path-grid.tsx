/** The docs home's three paths in: one card per job, each led by the link that job reaches for first. */
import { cn } from "@compozy/ui";
import { ArrowRight } from "@compozy/ui/icons";
import { FileText } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { FOCUS, LINE } from "@/components/docs/docs-classes";
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

export function PathGrid({ children }: { children: ReactNode }) {
  return <div className="docs-wide not-prose my-10 grid gap-5 lg:grid-cols-3">{children}</div>;
}

export function PathCard({ label, title, description, children }: PathCardProps) {
  return (
    <section className="flex flex-col rounded-[20px] border border-site-light-090 bg-site-light-025 p-6 md:p-7">
      <p className="m-0 font-mono text-[12.5px] text-brand-strong">{label}</p>
      <h3 className="mt-4 font-display text-[25px]/[1.15] font-normal tracking-[-0.02em] text-fg">
        {title}
      </h3>
      <p className="mt-3 flex-1 text-[14.5px]/[1.6] text-muted-foreground">{description}</p>
      <ul className={cn("not-prose mt-6 list-none border-t p-0 pt-2", LINE)}>{children}</ul>
    </section>
  );
}

export function PathLink({ href, icon, children, cta = false }: PathLinkProps) {
  const Icon = DOCS_ICONS[icon] ?? FileText;
  return (
    <li className="my-0!">
      <Link
        href={href}
        className={cn(
          "group flex w-full items-center gap-3 rounded-[8px] py-2.5 text-left text-[14.5px] transition-colors hover:text-fg",
          cta ? "text-fg" : "text-fg/70",
          FOCUS
        )}
      >
        <Icon
          aria-hidden
          className={cn("size-4 shrink-0", cta ? "text-brand-strong" : "text-subtle")}
        />
        {children}
        <ArrowRight
          className="ml-auto size-3.5 text-subtle transition-transform group-hover:translate-x-0.5 group-hover:text-fg"
          aria-hidden
        />
      </Link>
    </li>
  );
}
