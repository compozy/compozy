/** `run/extract-billing-client-3f9a0c12` → `run/extract-billi…3f9a0c12` at 26/8. */
export function middleTruncate(value: string, max: number, tail: number): string {
  const chars = [...value];
  if (chars.length <= max) return value;
  const keepTail = Math.min(tail, max - 2);
  const keepHead = max - 1 - keepTail;
  return `${chars.slice(0, keepHead).join("")}…${chars.slice(chars.length - keepTail).join("")}`;
}
