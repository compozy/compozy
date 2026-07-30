import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { MarketplaceEntryDetail } from "@/components/marketplace/marketplace-entry-detail";
import {
  findEntry,
  isMarketplaceKind,
  MARKETPLACE_KINDS,
  entriesForKind,
} from "@/lib/marketplace-catalog";
import { createPageMetadata } from "@/lib/site-config";

interface PageProps {
  params: Promise<{ kind: string; entryId: string }>;
}

export function generateStaticParams() {
  return MARKETPLACE_KINDS.flatMap(kind =>
    entriesForKind(kind).map(entry => ({ kind, entryId: entry.entry_id }))
  );
}

export async function generateMetadata(props: PageProps): Promise<Metadata> {
  const { kind, entryId } = await props.params;
  if (!isMarketplaceKind(kind)) notFound();
  const entry = findEntry(kind, entryId);
  if (!entry) notFound();

  return createPageMetadata({
    title: `${entry.name} — Marketplace`,
    description: entry.description,
    path: `/marketplace/${kind}/${entryId}`,
  });
}

export default async function MarketplaceEntryPage(props: PageProps) {
  const { kind, entryId } = await props.params;
  if (!isMarketplaceKind(kind)) notFound();
  const entry = findEntry(kind, entryId);
  if (!entry) notFound();

  return <MarketplaceEntryDetail kind={kind} entry={entry} />;
}
