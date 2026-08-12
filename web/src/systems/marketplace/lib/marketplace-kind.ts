import type { MarketplaceKind } from "../types";

export const MARKETPLACE_KIND_ORDER: readonly MarketplaceKind[] = ["skill", "mcp", "extension"];

export const MARKETPLACE_KIND_LABEL: Record<MarketplaceKind, string> = {
  skill: "Skills",
  extension: "Extensions",
  mcp: "MCPs",
};

export const MARKETPLACE_KIND_SINGULAR: Record<MarketplaceKind, string> = {
  skill: "skill",
  extension: "extension",
  mcp: "MCP server",
};

export function isMarketplaceKind(value: unknown): value is MarketplaceKind {
  return typeof value === "string" && MARKETPLACE_KIND_ORDER.includes(value as MarketplaceKind);
}
