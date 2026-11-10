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

/**
 * The marketing header and footer links (landing draft order). A link is current when the
 * pathname is its route or nested under it.
 */
export const MARKETING_NAV_LINKS: readonly { label: string; href: string }[] = [
  { label: "Docs", href: "/docs" },
  { label: "Marketplace", href: "/marketplace" },
  { label: "Blog", href: "/blog" },
  { label: "Changelog", href: "/changelog" },
  { label: "Download", href: "/download" },
] as const;

export function isCurrentSection(pathname: string, href: string): boolean {
  return pathname === href || pathname.startsWith(`${href}/`);
}
