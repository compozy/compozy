/**
 * The only thing that may survive a redacted answer: how long it was.
 *
 * Stream, replay, and the resolved pin all quote this exact phrase so a
 * length marker cannot drift into a second wording that looks like content.
 */
export function terminalRedactedInputCopy(characters: number): string {
  return `hidden input · ${characters} characters`;
}
