import { sessionBadgeSignal, type SessionBadgeSignal } from "./session-badge";
import type { SessionWaitingOn } from "./session-pull-request";

/**
 * Token classes for the badge dictionary's word tones, kept beside the
 * dictionary rather than inside a component that renders it, so every state
 * word shares one opinion about its tone.
 *
 * Classes are spelled out per tone rather than interpolated so Tailwind can see
 * every one of them.
 */

export const SESSION_TONE_WORD_CLASS: Record<SessionBadgeSignal["tone"], string> = {
  neutral: "text-subtle",
  brand: "text-brand",
  success: "text-success",
  warning: "text-warning",
  destructive: "text-destructive",
  info: "text-info",
};

/** Tone class for the state word beside a mark. */
export function sessionBadgeWordClass(
  badge: string | null | undefined,
  waitingOn?: SessionWaitingOn | null
): string {
  return SESSION_TONE_WORD_CLASS[sessionBadgeSignal(badge, waitingOn).tone];
}
