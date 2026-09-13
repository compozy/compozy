import Avatar from "boring-avatars";
import { Component, useState, type ReactNode } from "react";

import { cn, KindIcon } from "@compozy/ui";

import {
  marketplaceBrandKeyFor,
  marketplaceBrandRegistry,
} from "../lib/marketplace-brand-registry";

type MarketplaceEntryLogoSize = "sm" | "md" | "lg";

interface MarketplaceEntryLogoEntry {
  entry_id: string;
  name: string;
  icon?: string | null;
  install_slug?: string | null;
}

interface MarketplaceEntryLogoProps {
  entry: MarketplaceEntryLogoEntry;
  /** sm 24 (shelf) · md 40 (row) · lg 56 (detail lede). */
  size?: MarketplaceEntryLogoSize;
  className?: string;
}

const WELL_CLASS: Record<MarketplaceEntryLogoSize, string> = {
  sm: "size-(--size-catalog-logo) rounded-sm",
  md: "size-(--size-provider-logo-well) rounded-md",
  lg: "size-14 rounded-lg",
};

const ICON_CLASS: Record<MarketplaceEntryLogoSize, string> = {
  sm: "size-4",
  md: "size-7",
  lg: "size-10",
};

const BRAND_CLASS: Record<MarketplaceEntryLogoSize, string> = {
  sm: "size-3.25",
  md: "size-5",
  lg: "size-7",
};

const MONOGRAM_CLASS: Record<MarketplaceEntryLogoSize, string> = {
  sm: "text-mono-id",
  md: "text-card-title",
  lg: "text-xl",
};

const MARBLE_PIXELS: Record<MarketplaceEntryLogoSize, number> = { sm: 24, md: 40, lg: 56 };

const GRAPHEME_SEGMENTER = typeof Intl.Segmenter === "function" ? new Intl.Segmenter() : null;

function firstGrapheme(value: string): string {
  const trimmed = value.trim();
  if (trimmed === "") return "?";
  const first = GRAPHEME_SEGMENTER
    ? GRAPHEME_SEGMENTER.segment(trimmed)[Symbol.iterator]().next().value?.segment
    : undefined;
  return (first ?? Array.from(trimmed)[0] ?? "?").toLocaleUpperCase();
}

/**
 * Rung 4 guard: the marble generator is a pure function, so a throw is a bug — but the card must
 * still show something that identifies the entry, never a generic glyph.
 */
class MarketplaceLogoBoundary extends Component<
  { fallback: ReactNode; children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}

/**
 * Entry logo ladder: feed `icon` → brand mark from the shared inventory → Boring Avatars marble
 * tile seeded by the entry id → monogram only when the tile fails to render. The marble tile is
 * the one authorized identity color on a resting row (DESIGN-NOTES Q3).
 */
function MarketplaceEntryLogo({ entry, size = "md", className }: MarketplaceEntryLogoProps) {
  const [iconFailed, setIconFailed] = useState(false);
  const icon = entry.icon?.trim();
  const brandKey = marketplaceBrandKeyFor(entry);
  const wellClass = cn(
    "inline-flex shrink-0 items-center justify-center overflow-hidden text-fg-strong",
    WELL_CLASS[size],
    className
  );

  if (icon && !iconFailed) {
    return (
      <span
        aria-hidden="true"
        className={cn(wellClass, "bg-surface-glaze")}
        data-rung="icon"
        data-slot="marketplace-entry-logo"
      >
        <img
          alt=""
          className={cn("block rounded-xs object-contain", ICON_CLASS[size])}
          decoding="async"
          loading="lazy"
          onError={() => setIconFailed(true)}
          referrerPolicy="no-referrer"
          src={icon}
        />
      </span>
    );
  }

  if (brandKey) {
    return (
      <span
        aria-hidden="true"
        className={cn(wellClass, "bg-surface-glaze")}
        data-rung="brand"
        data-slot="marketplace-entry-logo"
      >
        <KindIcon
          className={cn("text-fg-strong", BRAND_CLASS[size])}
          data-slot="marketplace-entry-brand"
          kind={brandKey}
          registry={marketplaceBrandRegistry}
          size="md"
          tone="default"
        />
      </span>
    );
  }

  const monogram = (
    <span
      aria-hidden="true"
      className={cn(
        wellClass,
        "bg-surface-glaze font-sans font-semibold tracking-tight uppercase",
        MONOGRAM_CLASS[size]
      )}
      data-rung="monogram"
      data-slot="marketplace-entry-logo"
    >
      {firstGrapheme(entry.name)}
    </span>
  );

  return (
    <MarketplaceLogoBoundary fallback={monogram}>
      <span
        aria-hidden="true"
        className={cn(wellClass, "[&>svg]:size-full")}
        data-rung="marble"
        data-slot="marketplace-entry-logo"
      >
        <Avatar name={entry.entry_id} size={MARBLE_PIXELS[size]} square variant="marble" />
      </span>
    </MarketplaceLogoBoundary>
  );
}

export { MarketplaceEntryLogo };
export type { MarketplaceEntryLogoEntry, MarketplaceEntryLogoProps, MarketplaceEntryLogoSize };
