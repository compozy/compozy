import { Package, Plug, Puzzle, ShoppingBag, Wrench, type LucideIcon } from "lucide-react";

import type { MarketplaceKind } from "../types";

export interface MarketplaceKindConfig {
  kind: MarketplaceKind;
  label: string;
  singular: string;
  icon: LucideIcon;
  searchPlaceholder: string;
  teachingEmptyTitle: string;
  teachingEmptyBody: string;
  cliHint: string;
}

const KIND_CONFIG: Record<MarketplaceKind, MarketplaceKindConfig> = {
  skill: {
    kind: "skill",
    label: "Skills",
    singular: "skill",
    icon: Wrench,
    searchPlaceholder: "Search skills…",
    teachingEmptyTitle: "No skills installed yet",
    teachingEmptyBody: "Everything you install from the marketplace shows up here.",
    cliHint: "compozy skill install <slug>",
  },
  mcp: {
    kind: "mcp",
    label: "MCPs",
    singular: "MCP",
    icon: Plug,
    searchPlaceholder: "Search MCP servers…",
    teachingEmptyTitle: "No MCPs installed yet",
    teachingEmptyBody: "Everything you install from the marketplace shows up here.",
    cliHint: "compozy mcp install <entry>",
  },
  extension: {
    kind: "extension",
    label: "Extensions",
    singular: "extension",
    icon: Puzzle,
    searchPlaceholder: "Search extensions…",
    teachingEmptyTitle: "No extensions installed yet",
    teachingEmptyBody:
      "Install from the marketplace, a GitHub release, a git repository, or a local build.",
    cliHint: "compozy extension install github:owner/repo",
  },
};

export function marketplaceKindConfig(kind: MarketplaceKind): MarketplaceKindConfig {
  return KIND_CONFIG[kind];
}

export const MARKETPLACE_SCOPE_ICONS = {
  installed: Package,
  market: ShoppingBag,
} as const;
