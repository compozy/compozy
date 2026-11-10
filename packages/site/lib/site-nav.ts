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
