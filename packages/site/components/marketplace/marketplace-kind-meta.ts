import { FileCode, Plug, Puzzle, type LucideIcon } from "lucide-react";
import type { MarketplaceKind } from "@/lib/marketplace-catalog";

export type { LucideIcon } from "lucide-react";

export type MarketplaceKindMeta = {
  kind: MarketplaceKind;
  /** Singular noun for breadcrumbs and detail copy. */
  noun: string;
  title: string;
  description: string;
  icon: LucideIcon;
};

/**
 * Copy grounded in the daemon contract — the three `internal/marketplace` kinds, nothing else.
 *
 * This is presentation order (Skills → MCP servers → Extensions), matching the reference marketplace
 * in `docs/design/opendesign/site/`. `MARKETPLACE_KINDS` in `lib/marketplace-catalog.ts` keeps the
 * daemon's own kind order for data paths.
 */
export const MARKETPLACE_KIND_META: readonly MarketplaceKindMeta[] = [
  {
    kind: "skills",
    noun: "Skill",
    title: "Skills",
    description:
      "Procedural instructions an agent loads to do a job the same way every time. A skill installs into your runtime and is available to every session that needs it.",
    icon: FileCode,
  },
  {
    kind: "mcp",
    noun: "MCP server",
    title: "MCP servers",
    description:
      "Model Context Protocol servers the runtime installs, scopes, and supervises for your agents. The catalog identifies required fields; use --set or a Vault ref to supply them. Values are never stored in the catalog or shown here.",
    icon: Plug,
  },
  {
    kind: "extensions",
    noun: "Extension",
    title: "Extensions",
    description:
      "Packages that add resources, capabilities, and Host API actions to the runtime. Every artifact ships with a SHA-256 digest and a provenance tier — trust maps to real fields, nothing else.",
    icon: Puzzle,
  },
];

export function kindMeta(kind: MarketplaceKind): MarketplaceKindMeta {
  const meta = MARKETPLACE_KIND_META.find(entry => entry.kind === kind);
  if (!meta) {
    throw new Error(`unknown marketplace kind: ${kind}`);
  }
  return meta;
}

export function extensionTierLabel(tier: "official" | "community" | "unverified"): string {
  return `${tier.slice(0, 1).toUpperCase()}${tier.slice(1)}`;
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
