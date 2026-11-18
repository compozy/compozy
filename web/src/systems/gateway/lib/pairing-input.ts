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
  const digits = trimmed.replace(/[\s-]/g, "");
  if (/^\d{6}$/.test(digits)) return { manualCode: digits };
  return { artifact: trimmed };
}
