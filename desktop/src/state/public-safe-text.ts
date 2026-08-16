export function publicSafeText(value: unknown, fallback: string): string {
  if (typeof value !== "string") return fallback;
  const safe = Array.from(value, character => {
    const codePoint = character.codePointAt(0) ?? 0;
    return codePoint <= 31 || codePoint === 127 ? " " : character;
  })
    .join("")
    .trim()
    .slice(0, 512);
  return safe || fallback;
}
