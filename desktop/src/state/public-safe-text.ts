const SECRET_ASSIGNMENT =
  /\b(authorization|bearer|claim[_-]?token|token|password|secret|vault[_-]?(?:id|ref|reference))\b\s*[:=]\s*(?:bearer\s+)?[^\s,;]+/giu;
const BEARER_TOKEN = /\bbearer\s+[^\s,;]+/giu;

export function publicSafeText(value: unknown, fallback: string): string {
  if (typeof value !== "string") return fallback;
  const normalized = Array.from(value, character => {
    const codePoint = character.codePointAt(0) ?? 0;
    return codePoint <= 31 || codePoint === 127 ? " " : character;
  })
    .join("")
    .replace(SECRET_ASSIGNMENT, "$1=[redacted]")
    .replace(BEARER_TOKEN, "Bearer [redacted]")
    .trim();
  const safe = normalized.slice(0, 512);
  return safe || fallback;
}
