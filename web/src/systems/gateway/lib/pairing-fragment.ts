const PAIRING_FRAGMENT_KEY = "pair";

/**
 * A scanned pairing link carries its artifact in the URL **fragment**, never in
 * the query string or path.
 *
 * A fragment is not sent to the server, is not written to access logs, and is
 * not forwarded in a `Referer` header — which is what keeps a single-use
 * pairing secret out of every server-visible surface on its way to the browser
 * that redeems it. The artifact then travels exactly once more, in the redeem
 * request body.
 */
export function readPairingFragment(): string {
  if (typeof window === "undefined") return "";
  const hash = window.location.hash.replace(/^#/, "");
  if (hash === "") return "";
  return new URLSearchParams(hash).get(PAIRING_FRAGMENT_KEY)?.trim() ?? "";
}

/**
 * Strips the artifact from the address bar. Called before anything can leave
 * the page, so the secret never survives in history, a bookmark, or a shared
 * screenshot of the URL.
 */
export function clearPairingFragment(): void {
  if (typeof window === "undefined") return;
  const hash = window.location.hash.replace(/^#/, "");
  if (hash === "") return;
  const params = new URLSearchParams(hash);
  if (!params.has(PAIRING_FRAGMENT_KEY)) return;
  params.delete(PAIRING_FRAGMENT_KEY);
  const remaining = params.toString();
  const next = `${window.location.pathname}${window.location.search}${remaining ? `#${remaining}` : ""}`;
  window.history.replaceState(window.history.state, "", next);
}

/** Builds the scannable link for a freshly minted artifact. */
export function buildPairingLink(address: string, artifact: string): string {
  const base = address.replace(/\/+$/, "");
  const params = new URLSearchParams({ [PAIRING_FRAGMENT_KEY]: artifact });
  return `${base}/#${params.toString()}`;
}
