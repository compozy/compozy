/**
 * The one credential a redeem request carries. The daemon accepts either form,
 * and the discriminant keeps the request from ever carrying both.
 */
export type PairingCredential =
  | { artifact: string; manualCode?: undefined }
  | { manualCode: string; artifact?: undefined };

/**
 * Classifies what the operator typed or pasted into the redeem gate.
 *
 * Spaces and dashes are cosmetic in a six-digit code (`123 456`, `123-456`), so
 * they are dropped before the test. Anything that is not exactly six digits
 * after that stays an artifact or link, exactly as before. Blank input is null.
 */
export function classifyPairingInput(raw: string): PairingCredential | null {
  const trimmed = raw.trim();
  if (trimmed === "") return null;
  const value = pairingArtifactFromLink(trimmed) ?? trimmed;
  const digits = value.replace(/[\s-]/g, "");
  if (/^\d{6}$/.test(digits)) return { manualCode: digits };
  return { artifact: value };
}

/**
 * A pasted pairing link carries the artifact in the `pair` fragment, exactly as
 * the scanned link does. Redeeming the whole URL would send something the
 * daemon cannot decode, so the fragment value is extracted first. Only http(s)
 * URLs are treated as links; a bare artifact is never parsed as a URL.
 */
function pairingArtifactFromLink(value: string): string | null {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    return null;
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") return null;
  const hash = url.hash.replace(/^#/, "");
  const fromFragment = hash === "" ? "" : (new URLSearchParams(hash).get("pair") ?? "").trim();
  if (fromFragment !== "") return fromFragment;
  const fromQuery = (url.searchParams.get("pair") ?? "").trim();
  return fromQuery === "" ? null : fromQuery;
}
