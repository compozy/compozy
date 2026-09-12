// Adapted from nexu-io/open-design (Apache-2.0); see ../SOURCES.md.
export const PURPLE_HEXES = [
  // Tailwind violet / purple — the original AI-slop palette.
  "#a855f7",
  "#9333ea",
  "#7c3aed",
  "#6d28d9",
  "#581c87",
  "#8b5cf6",
  "#a78bfa",
  "#c4b5fd",
  "#ddd6fe",
  "#ede9fe",
  // Tailwind indigo — Refero's #1 reported AI tell. Common solid uses
  // (button fill, accent badge), not just gradients, are flagged
  // separately by `ai-default-indigo` below.
  "#6366f1",
  "#4f46e5",
  "#4338ca",
  "#3730a3",
  "#312e81",
  "#818cf8",
  "#a5b4fc",
  "#c7d2fe",
  "#e0e7ff",
  "#eef2ff",
];

// Blue / cyan stops used in the documented "blue→cyan two-stop trust
// gradient" cardinal sin. The purple-gradient rule above only catches
// gradients that contain a violet/indigo hex or the literal
// `purple`/`violet` keyword, so an artifact emitting
// `linear-gradient(90deg, #3b82f6, #06b6d4)` (or the keyword form
// `linear-gradient(90deg, blue, cyan)`) slipped past P0 even though
// `craft/anti-ai-slop.md` explicitly flags it. The `trust-gradient`
// rule below pairs these against each other to close the gap.
const TRUST_GRADIENT_BLUE_HEXES = [
  // Tailwind blue 500–900 + 400/300/200.
  "#3b82f6",
  "#2563eb",
  "#1d4ed8",
  "#1e40af",
  "#1e3a8a",
  "#60a5fa",
  "#93c5fd",
  "#bfdbfe",
  // Tailwind sky 400–700 — the same blue→cyan ramp under a different name.
  "#0ea5e9",
  "#0284c7",
  "#0369a1",
  "#38bdf8",
  "#7dd3fc",
];
const TRUST_GRADIENT_CYAN_HEXES = [
  // Tailwind cyan 500–900 + 400/300/200.
  "#06b6d4",
  "#0891b2",
  "#0e7490",
  "#155e75",
  "#164e63",
  "#22d3ee",
  "#67e8f9",
  "#a5f3fc",
];

// Subset of PURPLE_HEXES that constitute the canonical "default LLM
// accent" — even a single solid use is a tell. The DESIGN.md provides
// `var(--accent)`; if a brief truly needs indigo, the design system
// should encode it explicitly so we know it's intentional.
//
// These literals flag generic defaults; declared brand tokens are handled by css.ts.
export const AI_DEFAULT_INDIGO = [
  "#6366f1",
  "#4f46e5",
  "#4338ca",
  "#3730a3",
  "#8b5cf6",
  "#7c3aed",
  "#a855f7",
];

export const SLOP_EMOJI = [
  "✨",
  "🚀",
  "🎯",
  "⚡",
  "🔥",
  "💡",
  "📈",
  "🎨",
  "🛡️",
  "🌟",
  "💪",
  "🎉",
  "👋",
  "🙌",
  "✅",
  "⭐",
  "🏆",
];

// Simple sentinel words for invented-metric copy. Catching every claim is
// hopeless; we look for the canonical AI-startup phrasings.
export const INVENTED_METRIC_PATTERNS = [
  /\b10×\s+(faster|better|easier)\b/i,
  /\b100×\s+(faster|better)\b/i,
  /\b99\.\d+%\s+uptime\b/i,
  /\bzero[- ]downtime\b/i,
  /\b3×\s+more\s+(productive|efficient)\b/i,
];

export const FILLER_PATTERNS = [
  /\bfeature\s+(one|two|three|1|2|3)\b/i,
  /\blorem\s+ipsum\b/i,
  /\bdolor\s+sit\s+amet\b/i,
  /\bplaceholder\s+text\b/i,
  /\bsample\s+content\b/i,
];

// Display-face check: an h1 / h2 / h3 element whose `font-family` lands on
// Inter / Roboto / Arial / -apple-system without an actual serif before it.
// We check the `<style>` block specifically; inline styles are checked too.
export const DISPLAY_SANS_RE =
  /(?:h1|h2|h3|\.h-?(?:hero|xl|lg|md))[^{}]*\{[^}]*font-family\s*:\s*["']?(?:Inter|Roboto|Arial|-apple-system|system-ui|SF\s+Pro)/i;

export function clip(s: string): string {
  if (!s) return "";
  const trimmed = s.replace(/\s+/g, " ").trim();
  return trimmed.length > 200 ? trimmed.slice(0, 197) + "…" : trimmed;
}

export function escapeRe(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

// Scan every `linear-gradient(...)` body for a blue→cyan two-stop
// trust gradient. Returns the first matching gradient text or `null`.
// The check accepts either Tailwind blue/sky/cyan hex stops or the
// literal `blue`/`cyan` keywords, so both
// `linear-gradient(90deg, #3b82f6, #06b6d4)` and
// `linear-gradient(90deg, blue, cyan)` fire P0.
export function detectBlueCyanTrustGradient(html: string): string | null {
  const re = /linear-gradient\([^)]*\)/gi;
  let m;
  while ((m = re.exec(html)) !== null) {
    const grad = m[0].toLowerCase();
    const hasBlue =
      TRUST_GRADIENT_BLUE_HEXES.some(h => grad.includes(h.toLowerCase())) || /\bblue\b/.test(grad);
    const hasCyan =
      TRUST_GRADIENT_CYAN_HEXES.some(h => grad.includes(h.toLowerCase())) || /\bcyan\b/.test(grad);
    if (hasBlue && hasCyan) return m[0];
  }
  return null;
}
