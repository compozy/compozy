import { bridgeKindIconRegistry, type KindIconRegistry } from "@compozy/ui";
import { ClaudeLogo, CursorLogo, GeminiLogo, OpenAILogo, VercelLogo } from "@compozy/ui/logos";
import { createElement, type SVGProps } from "react";

/**
 * Rung 2 of the entry logo ladder: a brand mark from the shared inventory keyed by `entry_id`, then
 * by the tail of `install_slug`. Every entry carries a real brand mark — a Lucide fallback would be
 * a generic glyph, which the card never shows.
 */
const marketplaceBrandRegistry = {
  ...bridgeKindIconRegistry,
  claude: { brand: ClaudeLogo },
  cursor: { brand: CursorLogo },
  gemini: { brand: GeminiLogo },
  openai: {
    render: (props: SVGProps<SVGSVGElement>) =>
      createElement(OpenAILogo, { ...props, mode: "dark" }),
  },
  vercel: { brand: VercelLogo },
} satisfies KindIconRegistry;

type MarketplaceBrandKey = keyof typeof marketplaceBrandRegistry;

function normalizeBrandKey(value: string | null | undefined): string {
  return value?.trim().toLowerCase() ?? "";
}

/** Resolves the brand key for a listing, or `null` when no shipped brand mark applies. */
export function marketplaceBrandKeyFor(entry: {
  entry_id: string;
  install_slug?: string | null;
}): MarketplaceBrandKey | null {
  const candidates = [entry.entry_id, entry.install_slug?.split("/").pop()]
    .map(normalizeBrandKey)
    .filter(candidate => candidate !== "");
  for (const candidate of candidates) {
    if (Object.hasOwn(marketplaceBrandRegistry, candidate)) {
      return candidate as MarketplaceBrandKey;
    }
  }
  return null;
}

export { marketplaceBrandRegistry };
export type { MarketplaceBrandKey };
