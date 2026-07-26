import { Box, Package, Plug, Puzzle, ShoppingBag, Wrench, type LucideIcon } from "lucide-react";

import type { MarketplaceKind } from "../types";

export interface MarketplaceKindConfig {
  kind: MarketplaceKind;
  label: string;
  singular: string;
  icon: LucideIcon;
  searchPlaceholder: string;
  verb: string;
  installedNoun: "installed" | "active";
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
    verb: "Install",
    installedNoun: "installed",
    teachingEmptyTitle: "No skills installed yet",
    teachingEmptyBody: "Everything you install from the marketplace shows up here.",
    cliHint: "agh skill install <slug>",
  },
  mcp: {
    kind: "mcp",
    label: "MCPs",
    singular: "MCP",
    icon: Plug,
    searchPlaceholder: "Search MCP servers…",
    verb: "Install",
    installedNoun: "installed",
    teachingEmptyTitle: "No MCPs installed yet",
    teachingEmptyBody: "Everything you install from the marketplace shows up here.",
    cliHint: "agh mcp install <name>",
  },
  extension: {
    kind: "extension",
    label: "Extensions",
    singular: "extension",
    icon: Puzzle,
    searchPlaceholder: "Search extensions…",
    verb: "Install",
    installedNoun: "installed",
    teachingEmptyTitle: "No extensions installed yet",
    teachingEmptyBody: "Everything you install from the marketplace shows up here.",
    cliHint: "agh extension install <path-or-slug>",
  },
  bundle: {
    kind: "bundle",
    label: "Bundles",
    singular: "bundle",
    icon: Box,
    searchPlaceholder: "Search bundles…",
    verb: "Activate",
    installedNoun: "active",
    teachingEmptyTitle: "No bundles active yet",
    teachingEmptyBody: "A bundle activates a curated set of capabilities in one step.",
    cliHint: "agh bundle activate",
  },
};

export function marketplaceKindConfig(kind: MarketplaceKind): MarketplaceKindConfig {
  return KIND_CONFIG[kind];
}

export const MARKETPLACE_SCOPE_ICONS = {
  installed: Package,
  market: ShoppingBag,
} as const;
