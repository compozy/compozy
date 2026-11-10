/** The detail page frame shared by catalog entries and bundled extensions: identity, an install aside, neighbours. */
import { cn } from "@compozy/ui";
import { ArrowLeft, ArrowUpRight, Download, ShieldCheck } from "@compozy/ui/icons";
import Link from "next/link";
import type { ReactNode } from "react";

import { EntryCard } from "@/components/marketplace/entry-card";
import { CONTAINER, HAIRLINE } from "@/components/site/layout";
import { H1, H3, InstallCommand, PrimaryCta, Small } from "@/components/site/primitives";
import { hostnameOf, type MarketplaceCard } from "@/lib/marketplace-view";

export function Row({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className={cn("flex items-baseline justify-between gap-6 border-t py-3", HAIRLINE)}>
      <dt className="text-[14px] text-subtle">{term}</dt>
      <dd className="m-0 min-w-0 truncate text-right text-[14px] text-fg">{children}</dd>
    </div>
  );
}

export function HomepageLink({ href }: { href: string }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="inline-flex items-center gap-1 text-fg underline-offset-4 hover:underline"
    >
      {hostnameOf(href)} <ArrowUpRight aria-hidden className="size-3.5" />
    </a>
  );
}

/** The install aside: a title, one line, a command, Download, the facts, and the confirmation note. */
export function InstallAside({
  title,
  lead,
  command,
  facts,
  note,
}: {
  title: string;
  lead: ReactNode;
  command: ReactNode;
  facts: ReactNode;
  note: string;
}) {
  return (
    <aside className="lg:sticky lg:top-24 lg:self-start">
      <div className="rounded-[20px] border border-site-light-100 bg-site-light-030 p-6">
        <H3 className="md:text-[22px]">{title}</H3>
        {lead}
        {command}
        <div className="mt-4 [&_a]:w-full [&_button]:w-full">
          <PrimaryCta icon={Download} href="/download">
            Download CompozyOS
          </PrimaryCta>
        </div>
        <dl className="m-0 mt-6">{facts}</dl>
        <Small className="mt-4 flex gap-2">
          <ShieldCheck aria-hidden className="mt-0.5 size-3.5 shrink-0" />
          {note}
        </Small>
      </div>
    </aside>
  );
}

/** The install command at full width, wrapping long commands instead of truncating them. */
export function DetailCommand({ command }: { command: string }) {
  return (
    <InstallCommand
      command={command}
      className="mt-5 flex h-auto min-h-[52px] w-full justify-between rounded-[16px] py-2 [&_code]:flex-1 [&_code]:whitespace-normal [&_code]:[overflow-wrap:anywhere]"
    />
  );
}

export function DetailFrame({
  logo,
  name,
  byline,
  description,
  aside,
  children,
  relatedTitle,
  related,
}: {
  logo: ReactNode;
  name: string;
  byline: ReactNode;
  description: string;
  aside: ReactNode;
  children?: ReactNode;
  relatedTitle: string;
  related: readonly MarketplaceCard[];
}) {
  return (
    <div className={cn(CONTAINER, "pt-[120px] pb-28 md:pt-[136px]")}>
      <Link
        href="/marketplace"
        className="inline-flex cursor-pointer items-center gap-2 bg-transparent p-0 text-[14px] text-muted-foreground outline-none transition-colors hover:text-fg focus-visible:text-fg"
      >
        <ArrowLeft aria-hidden className="size-4" /> Marketplace
      </Link>
      <div className="mt-10 grid gap-12 lg:grid-cols-[minmax(0,1fr)_380px] lg:gap-16">
        <div className="min-w-0">
          {logo}
          <div className="mt-7 [&_h1]:md:text-[64px] [&_h1]:md:leading-[1.03]">
            <H1>{name}</H1>
          </div>
          <p className="mt-4 mb-0 flex flex-wrap items-center gap-3 text-[15px] text-muted-foreground">
            {byline}
          </p>
          <p className="mt-9 mb-0 max-w-[68ch] text-[18px]/[1.65] text-pretty text-fg/85">
            {description}
          </p>
          {children}
        </div>
        {aside}
      </div>
      {related.length ? (
        <div className={cn("mt-24 border-t pt-14", HAIRLINE)}>
          <H3>{relatedTitle}</H3>
          <div className="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {related.map(item => (
              <EntryCard key={item.id} entry={item} />
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}

/** A titled block of contents under the description, in the same hairline rhythm as the aside. */
export function ContentsBlock({
  title,
  children,
  className,
}: {
  title: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={cn("mt-14 max-w-[68ch]", className)}>
      <h2 className="m-0 mb-4 text-[18px] font-medium text-fg">{title}</h2>
      {children}
    </section>
  );
}
