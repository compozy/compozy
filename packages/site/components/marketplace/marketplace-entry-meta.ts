import type { ExtensionEntry } from "@/lib/marketplace-catalog";

/**
 * Presentation helpers grounded in the v3 feed contract (`lib/marketplace-catalog-schema.ts`).
 * Every value here is derived from a real entry field; nothing is a popularity signal, because no
 * such field exists in the feed.
 */

export type ExtensionTier = ExtensionEntry["tier"];
export type ExtensionInput = NonNullable<ExtensionEntry["inputs"]>[number];

export const TIER_HINTS: Record<ExtensionTier, string> = {
  official: "First-party, shipped from the CompozyOS repository.",
  community: "Community-published; review the repository before installing.",
  unverified: "Not reviewed by CompozyOS; verify the artifact yourself.",
};

export const FORMAT_LABELS: Record<NonNullable<ExtensionEntry["format"]>, string> = {
  compozy: "CompozyOS package",
  "agent-plugin": "Agent plugin",
};

export const INPUT_TYPE_LABELS: Record<ExtensionInput["type"], string> = {
  string: "Text",
  identifier: "Identifier",
  boolean: "Boolean",
  secret: "Secret",
};

export function extensionTierLabel(tier: ExtensionTier): string {
  return `${tier.slice(0, 1).toUpperCase()}${tier.slice(1)}`;
}

export function versionLabel(version: string): string {
  return version.startsWith("v") ? version : `v${version}`;
}

export function shortDigest(digest: string): string {
  return `sha256 · ${digest.slice(0, 8)}…`;
}

/**
 * The faint word after the name on a card: the board shows "community · author" only for
 * third-party tiers; an official entry renders nothing there.
 */
export function provenanceWord(entry: ExtensionEntry): string | null {
  if (entry.tier === "official") return null;
  const author = entry.author?.trim();
  return author ? `${entry.tier} · ${author}` : entry.tier;
}

export function inputSummary(inputs: ExtensionEntry["inputs"]): string | null {
  const count = inputs?.length ?? 0;
  if (!inputs || count === 0) return null;
  const required = inputs.filter(input => input.required).length;
  const noun = count === 1 ? "input" : "inputs";
  if (required === 0) return `${count} optional ${noun}`;
  if (required === count) return `${count} required ${noun}`;
  return `${count} ${noun} · ${required} required`;
}

export function bindingLabel(binding: ExtensionInput["binding"]): string {
  return binding.type === "env" ? `env ${binding.name}` : `?${binding.name}=`;
}

export function formatFeedDate(value: string | undefined): string | null {
  if (!value) return null;
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return null;
  return parsed.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  });
}

/** "Updated" wins over "Published" when the feed carries both, as the daemon detail does. */
export function feedDateLine(entry: ExtensionEntry): { verb: string; date: string } | null {
  const date = formatFeedDate(entry.updated_at ?? entry.published_at);
  if (!date) return null;
  return { verb: entry.updated_at ? "Updated" : "Published", date };
}
