import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { MarketplaceEntryDetail } from "@/components/marketplace/marketplace-entry-detail";
import { findEntry, extensionEntries, marketplaceEntryPath } from "@/lib/marketplace-catalog";
import { createPageMetadata } from "@/lib/site-config";

interface PageProps {
  params: Promise<{ entryId: string }>;
}

export function generateStaticParams() {
  return extensionEntries.map(entry => ({ entryId: entry.entry_id }));
}

export async function generateMetadata(props: PageProps): Promise<Metadata> {
  const { entryId } = await props.params;
  const entry = findEntry(entryId);
  if (!entry) notFound();

  return createPageMetadata({
    title: `${entry.name} — Marketplace`,
    description: entry.description,
    path: marketplaceEntryPath(entry),
  });
}

export default async function MarketplaceEntryPage(props: PageProps) {
  const { entryId } = await props.params;
  const entry = findEntry(entryId);
  if (!entry) notFound();

  return <MarketplaceEntryDetail entry={entry} />;
}
