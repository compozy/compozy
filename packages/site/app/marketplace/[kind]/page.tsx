import { Eyebrow } from "@compozy/ui";
import { ArrowLeft } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { MarketplaceEntryCard } from "@/components/marketplace/marketplace-entry-card";
import { kindMeta } from "@/components/marketplace/marketplace-kind-meta";
import { entriesForKind, isMarketplaceKind, MARKETPLACE_KINDS } from "@/lib/marketplace-catalog";
import { createPageMetadata } from "@/lib/site-config";

interface PageProps {
  params: Promise<{ kind: string }>;
}

export function generateStaticParams() {
  return MARKETPLACE_KINDS.map(kind => ({ kind }));
}

export async function generateMetadata(props: PageProps): Promise<Metadata> {
  const { kind } = await props.params;
  if (!isMarketplaceKind(kind)) notFound();
  const meta = kindMeta(kind);

  return createPageMetadata({
    title: `${meta.title} — Marketplace`,
    description: meta.description,
    path: `/marketplace/${kind}`,
  });
}

export default async function MarketplaceKindPage(props: PageProps) {
  const { kind } = await props.params;
  if (!isMarketplaceKind(kind)) notFound();
  const meta = kindMeta(kind);
  const entries = entriesForKind(kind);

  return (
    <main id="main-content" className="mx-auto w-full max-w-site-layout-width px-4 pt-12 pb-20">
      <Link
        href="/marketplace"
        className="inline-flex items-center gap-1.5 text-small-body font-medium text-muted transition-colors hover:text-accent"
      >
        <ArrowLeft aria-hidden className="size-3.5" />
        Marketplace
      </Link>

      <header className="mt-6 max-w-[62ch]">
        <Eyebrow className="text-muted">Kind</Eyebrow>
        <h1 className="mt-3 flex items-baseline gap-3 text-3xl font-semibold tracking-[-0.02em] text-fg">
          {meta.title}
          <span className="font-mono text-base font-normal text-subtle">{entries.length}</span>
        </h1>
        <p className="mt-4 text-base leading-relaxed text-muted">{meta.description}</p>
      </header>

      <div className="mt-10 grid gap-4 lg:grid-cols-2">
        {entries.map(entry => (
          <MarketplaceEntryCard key={entry.entry_id} kind={kind} entry={entry} />
        ))}
      </div>
    </main>
  );
}
