"use client";

import { bridgeKindIconRegistry, CatalogCard, cn, KindIcon } from "@compozy/ui";
import Avatar from "boring-avatars";
import { Component, useState, type ReactNode } from "react";
import type { ExtensionEntry } from "@/lib/marketplace-catalog";

// Feed icon → shared brand mark → seeded marble tile → monogram on render failure.

type MarketplaceEntryLogoSize = "md" | "lg";

type LogoEntry = Pick<ExtensionEntry, "entry_id" | "name" | "icon" | "install_slug">;

const WELL_CLASS: Record<MarketplaceEntryLogoSize, string> = {
  md: "rounded-md",
  lg: "size-14 rounded-lg",
};

const ICON_CLASS: Record<MarketplaceEntryLogoSize, string> = {
  md: "size-7",
  lg: "size-10",
};

const BRAND_CLASS: Record<MarketplaceEntryLogoSize, string> = {
  md: "size-5",
  lg: "size-7",
};

const MONOGRAM_CLASS: Record<MarketplaceEntryLogoSize, string> = {
  md: "text-card-title",
  lg: "text-xl",
};

const GRAPHEME_SEGMENTER = typeof Intl.Segmenter === "function" ? new Intl.Segmenter() : null;

function firstGrapheme(value: string): string {
  const trimmed = value.trim();
  if (trimmed === "") return "?";
  const first = GRAPHEME_SEGMENTER
    ? GRAPHEME_SEGMENTER.segment(trimmed)[Symbol.iterator]().next().value?.segment
    : undefined;
  return (first ?? Array.from(trimmed)[0] ?? "?").toLocaleUpperCase();
}

function brandKeyFor(entry: LogoEntry): string | null {
  const candidates = [entry.entry_id, entry.install_slug.split("/").pop()]
    .map(value => value?.trim().toLowerCase() ?? "")
    .filter(value => value !== "");
  for (const candidate of candidates) {
    if (Object.hasOwn(bridgeKindIconRegistry, candidate)) return candidate;
  }
  return null;
}

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

export function MarketplaceEntryLogo({
  entry,
  size = "md",
  className,
}: {
  entry: LogoEntry;
  size?: MarketplaceEntryLogoSize;
  className?: string;
}) {
  const [failedIcon, setFailedIcon] = useState<string>();
  const icon = entry.icon?.trim();
  const brandKey = brandKeyFor(entry);
  const wellClass = cn(
    "overflow-hidden border border-line bg-elevated text-fg-strong",
    WELL_CLASS[size],
    className
  );

  if (icon && icon !== failedIcon) {
    return (
      <CatalogCard.Logo tone="neutral" size="lg" className={wellClass} data-rung="icon">
        <img
          alt=""
          className={cn("block rounded-xs object-contain", ICON_CLASS[size])}
          decoding="async"
          loading="lazy"
          onError={() => setFailedIcon(icon)}
          referrerPolicy="no-referrer"
          src={icon}
        />
      </CatalogCard.Logo>
    );
  }

  if (brandKey) {
    return (
      <CatalogCard.Logo tone="neutral" size="lg" className={wellClass} data-rung="brand">
        <KindIcon
          className={cn("text-fg-strong", BRAND_CLASS[size])}
          kind={brandKey}
          registry={bridgeKindIconRegistry}
          size="md"
          tone="default"
        />
      </CatalogCard.Logo>
    );
  }

  const monogram = (
    <CatalogCard.Logo
      tone="neutral"
      size="lg"
      className={cn(wellClass, "font-sans font-semibold tracking-tight", MONOGRAM_CLASS[size])}
      data-rung="monogram"
    >
      {firstGrapheme(entry.name)}
    </CatalogCard.Logo>
  );

  return (
    <MarketplaceLogoBoundary fallback={monogram} key={entry.entry_id}>
      <CatalogCard.Logo
        tone="neutral"
        size="lg"
        className={cn(wellClass, "[&>svg]:size-full")}
        data-rung="marble"
      >
        <Avatar name={entry.entry_id} size={size === "lg" ? 56 : 40} square variant="marble" />
      </CatalogCard.Logo>
    </MarketplaceLogoBoundary>
  );
}
