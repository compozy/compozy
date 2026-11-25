/**
 * The marketing header and footer links (landing draft order). A link is current when the
 * pathname is its route or nested under it.
 */
export const MARKETING_NAV_LINKS: readonly { label: string; href: string }[] = [
  { label: "Docs", href: "/docs" },
  { label: "Marketplace", href: "/marketplace" },
  { label: "Pricing", href: "/pricing" },
  { label: "Blog", href: "/blog" },
  { label: "Changelog", href: "/changelog" },
  { label: "Download", href: "/download" },
] as const;

export const LEGAL_NAV_LINKS: readonly { label: string; href: string }[] = [
  { label: "Terms", href: "/legal/terms" },
  { label: "Privacy", href: "/legal/privacy" },
  { label: "Refunds", href: "/legal/refunds" },
] as const;

export function isCurrentSection(pathname: string, href: string): boolean {
  return pathname === href || pathname.startsWith(`${href}/`);
}
