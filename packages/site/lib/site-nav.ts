/**
 * Single source for the five top-level site destinations (spec D1/D12). Consumed by
 * `lib/layout.shared.tsx` (Fumadocs `baseOptions.links`) and `components/site/home-header.tsx`.
 */
export type SiteNavLink = {
  label: string;
  href: string;
  /** Fumadocs link activation mode: exact URL or any nested path. */
  active: "url" | "nested-url";
};

export const SITE_NAV_LINKS: readonly SiteNavLink[] = [
  { label: "Home", href: "/", active: "url" },
  { label: "Docs", href: "/docs", active: "nested-url" },
  { label: "Marketplace", href: "/marketplace", active: "nested-url" },
  { label: "Blog", href: "/blog", active: "nested-url" },
  { label: "Changelog", href: "/changelog", active: "nested-url" },
] as const;
