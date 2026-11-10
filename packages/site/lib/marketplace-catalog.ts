import type { z } from "zod";
import extensionsFeed from "../../../catalog/v3/extensions.json";
import { extensionEntrySchema, extensionFeedSchema } from "./marketplace-catalog-schema";

export type ExtensionEntry = z.infer<typeof extensionEntrySchema>;

export const MARKETPLACE_SEARCH_COMMAND = "compozy marketplace search";
export const extensionEntries: ExtensionEntry[] = extensionFeedSchema.parse(extensionsFeed).entries;

export function findEntry(entryId: string): ExtensionEntry | undefined {
  return extensionEntries.find(entry => entry.entry_id === entryId);
}

export function marketplaceEntryPath(entry: Pick<ExtensionEntry, "entry_id">): string {
  return `/marketplace/${encodeURIComponent(entry.entry_id)}`;
}

export function marketplaceSearchCommand(entry: ExtensionEntry): string {
  return `${MARKETPLACE_SEARCH_COMMAND} ${entry.entry_id}`;
}

export function installCommand(entry: ExtensionEntry): string {
  return `compozy extension install ${entry.install_slug}`;
}
