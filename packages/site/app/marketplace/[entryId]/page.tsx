import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { EntryDetail } from "@/components/marketplace/entry-detail";
import { PluginDetail } from "@/components/marketplace/plugin-detail";
import { extensionEntries, findEntry, marketplaceEntryPath } from "@/lib/marketplace-catalog";
import {
  findPlugin,
  marketplacePluginPath,
  marketplacePlugins,
  pluginDisplayName,
} from "@/lib/marketplace-plugins";
import { createPageMetadata } from "@/lib/site-config";

interface PageProps {
  params: Promise<{ entryId: string }>;
}

export function generateStaticParams() {
  return [
    ...extensionEntries.map(entry => ({ entryId: entry.entry_id })),
    ...marketplacePlugins.map(entry => ({ entryId: entry.id })),
  ];
}

/** Next hands the segment back percent-encoded when it holds a reserved character. */
function lookup(entryId: string) {
  const id = decodeURIComponent(entryId);
  const entry = findEntry(id);
  if (entry) return { kind: "extension" as const, entry };
  const plugin = findPlugin(id);
  if (plugin) return { kind: "plugin" as const, entry: plugin };
  notFound();
}

export async function generateMetadata(props: PageProps): Promise<Metadata> {
  const found = lookup((await props.params).entryId);
  return found.kind === "extension"
    ? createPageMetadata({
        title: `${found.entry.name} — Marketplace`,
        description: found.entry.description,
        path: marketplaceEntryPath(found.entry),
      })
    : createPageMetadata({
        title: `${pluginDisplayName(found.entry.plugin)} — Marketplace`,
        description: found.entry.plugin.description ?? "",
        path: marketplacePluginPath(found.entry),
      });
}

export default async function MarketplaceEntryPage(props: PageProps) {
  const found = lookup((await props.params).entryId);
  return found.kind === "extension" ? (
    <EntryDetail entry={found.entry} />
  ) : (
    <PluginDetail entry={found.entry} />
  );
}
