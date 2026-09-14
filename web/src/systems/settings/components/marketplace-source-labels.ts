import type { MarketplaceSource } from "@/systems/marketplace";

export type MarketplaceSourceOriginKind = "feed" | "folder" | "repository";

export interface MarketplaceSourceOrigin {
  kind: MarketplaceSourceOriginKind;
  /** Row label: Feed · Folder · Repository. */
  label: string;
  /** The normalized origin, shown as the daemon stores it minus a `file://` scheme. */
  value: string;
}

/**
 * Every string here restates a field the daemon reported. The origin is the immutable normalized
 * `source`; only its scheme decides whether the row calls it a feed, a folder, or a repository.
 */
export function marketplaceSourceOrigin(source: MarketplaceSource): MarketplaceSourceOrigin {
  const value = source.source.trim();
  if (source.kind === "feed") return { kind: "feed", label: "Feed", value };
  if (value.startsWith("file://")) {
    return { kind: "folder", label: "Folder", value: value.slice("file://".length) };
  }
  if (value.startsWith("/") || value.startsWith("~")) {
    return { kind: "folder", label: "Folder", value };
  }
  return { kind: "repository", label: "Repository", value };
}

export function marketplaceSourceTestId(name: string): string {
  return `settings-page-marketplace-source-${name}`;
}

/** "6 plugins" for a marketplace, "20 extensions" for the feed. */
export function marketplaceSourceCountLabel(source: MarketplaceSource): string {
  const noun = source.kind === "feed" ? "extension" : "plugin";
  return source.plugins === 1 ? `1 ${noun}` : `${source.plugins} ${noun}s`;
}

/** "6 listed · 6 can be installed" — m < n only when the daemon dropped a plugin at decode. */
export function marketplaceSourcePluginsLine(source: MarketplaceSource): string {
  return `${source.plugins} listed · ${source.installable} can be installed`;
}

/** The daemon has read this source at least once; counts are a measurement, not a zero. */
export function marketplaceSourceWasRead(source: MarketplaceSource): boolean {
  return typeof source.last_read_at === "string" && source.last_read_at !== "";
}

export function marketplaceSourceDegraded(source: MarketplaceSource): boolean {
  return source.state === "degraded";
}

/** Plain sentence first; the reason (error class · error) renders separately in micro mono. */
export function marketplaceSourceDegradedSentence(
  source: MarketplaceSource,
  origin: MarketplaceSourceOrigin
): string {
  const noun = origin.kind === "feed" ? "feed" : origin.kind;
  return source.error_class === "source_unreachable"
    ? `The ${noun} is not reachable right now.`
    : `CompozyOS could not refresh this ${noun}.`;
}

export function marketplaceSourceReason(source: MarketplaceSource): string | null {
  const parts = [source.error_class?.trim(), source.error?.trim()].filter((part): part is string =>
    Boolean(part)
  );
  return parts.length > 0 ? parts.join(" · ") : null;
}
