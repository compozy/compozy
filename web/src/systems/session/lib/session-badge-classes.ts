import { sessionBadgeSignal, type SessionBadgeSignal } from "./session-badge";

/**
 * Token classes for the badge dictionary's tones, kept beside the dictionary
 * rather than inside the component that renders it: the state word, the row
 * mark, and the 18 px roundel all need the same tone and must never drift into
 * separate opinions about it.
 *
 * Classes are spelled out per tone rather than interpolated so Tailwind can see
 * every one of them.
 */

export const SESSION_TONE_WORD_CLASS: Record<SessionBadgeSignal["tone"], string> = {
  neutral: "text-subtle",
  accent: "text-accent",
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
  info: "text-info",
};

export const SESSION_TONE_ROUNDEL_CLASS: Record<SessionBadgeSignal["tone"], string> = {
  neutral: "bg-neutral-tint text-neutral-ink",
  accent: "bg-accent-tint text-accent",
  success: "bg-success-tint text-success",
  warning: "bg-warning-tint text-warning",
  danger: "bg-danger-tint text-danger",
  info: "bg-info-tint text-info",
};

/** Tone class for the state word beside a mark. */
export function sessionBadgeWordClass(badge: string | null | undefined): string {
  return SESSION_TONE_WORD_CLASS[sessionBadgeSignal(badge).tone];
}
