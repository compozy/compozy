/** Absolute http(s) URLs leave the site in a new tab; everything else is a client-side route. */
export function isExternalHref(href: string) {
  return /^https?:\/\//.test(href);
}
