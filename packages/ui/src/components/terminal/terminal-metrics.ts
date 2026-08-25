/**
 * Type metrics for the emulator, read from CSS rather than hardcoded.
 *
 * The emulator needs numbers, but the numbers belong to the design system. The
 * consumer styles the container with tokens and this module translates whatever
 * CSS resolved into the shape the emulator wants, so the primitive carries no
 * product-specific type scale of its own.
 */

export interface TerminalTypeMetrics {
  fontFamily: string;
  fontSize: number;
  lineHeight: number;
  letterSpacing: number;
}

const FALLBACK_FONT_FAMILY = "monospace";
const FALLBACK_FONT_SIZE = 12;
const FALLBACK_LINE_HEIGHT = 1.5;

export function readTerminalMetrics(element: HTMLElement): TerminalTypeMetrics {
  const view = element.ownerDocument?.defaultView;
  if (!view) {
    return {
      fontFamily: FALLBACK_FONT_FAMILY,
      fontSize: FALLBACK_FONT_SIZE,
      lineHeight: FALLBACK_LINE_HEIGHT,
      letterSpacing: 0,
    };
  }
  const computed = view.getComputedStyle(element);
  const fontSize = parsePixels(computed.fontSize) ?? FALLBACK_FONT_SIZE;
  return {
    fontFamily: computed.fontFamily.trim() || FALLBACK_FONT_FAMILY,
    fontSize,
    lineHeight: parseLineHeight(computed.lineHeight, fontSize),
    letterSpacing: parsePixels(computed.letterSpacing) ?? 0,
  };
}

/** True when two metric sets would lay the grid out identically. */
export function terminalMetricsEqual(
  left: TerminalTypeMetrics,
  right: TerminalTypeMetrics
): boolean {
  return (
    left.fontFamily === right.fontFamily &&
    left.fontSize === right.fontSize &&
    left.lineHeight === right.lineHeight &&
    left.letterSpacing === right.letterSpacing
  );
}

function parsePixels(value: string): number | null {
  const parsed = Number.parseFloat(value);
  return Number.isFinite(parsed) ? parsed : null;
}

/** The emulator wants a unitless multiplier; CSS may answer in either form. */
function parseLineHeight(value: string, fontSize: number): number {
  const trimmed = value.trim();
  if (trimmed === "" || trimmed === "normal") return FALLBACK_LINE_HEIGHT;
  const parsed = Number.parseFloat(trimmed);
  if (!Number.isFinite(parsed) || parsed <= 0) return FALLBACK_LINE_HEIGHT;
  if (trimmed.endsWith("px")) {
    return fontSize > 0 ? parsed / fontSize : FALLBACK_LINE_HEIGHT;
  }
  return parsed;
}
