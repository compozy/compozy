import type { SessionDerivePreview } from "../adapters/session-derive-api";
import { formatContextBytes } from "./context-format";

/**
 * View model of the shared context line in the Continue and Fork dialogs. Every
 * number comes from the daemon's preview; nothing is estimated client-side.
 */
export type SessionDerivePreviewView =
  | { state: "measuring" }
  | { state: "error" }
  | {
      state: "ready" | "truncated";
      /** Bold lead: `Carries over 42 messages` / `Carries over 30 of 42 messages`. */
      headline: string;
      size: string;
      /** Truncated only: `12 earlier messages omitted to fit the context budget.` */
      omitted: string | null;
      /** A running source turn is excluded from what travels. */
      turnInProgress: boolean;
    };

function messages(count: number): string {
  return count === 1 ? "1 message" : `${count.toLocaleString()} messages`;
}

export function sessionDerivePreviewView(
  preview: SessionDerivePreview | undefined,
  status: "pending" | "error" | "success"
): SessionDerivePreviewView {
  if (status === "error") return { state: "error" };
  if (status === "pending" || !preview) return { state: "measuring" };
  const carried = Math.max(0, preview.message_count);
  const omitted = preview.truncated ? Math.max(0, preview.omitted_count) : 0;
  const size = formatContextBytes(Math.max(0, preview.replay_bytes));
  const turnInProgress = preview.source_turn_in_progress;
  if (omitted === 0) {
    return {
      state: "ready",
      headline: `Carries over ${messages(carried)}`,
      size,
      omitted: null,
      turnInProgress,
    };
  }
  const total = carried + omitted;
  return {
    state: "truncated",
    headline: `Carries over ${carried.toLocaleString()} of ${messages(total)}`,
    size,
    omitted: `${omitted.toLocaleString()} earlier ${omitted === 1 ? "message" : "messages"} omitted to fit the context budget.`,
    turnInProgress,
  };
}

export interface SessionDeriveRoute {
  provider: string;
  model: string;
  command_fingerprint?: string;
}

export interface SessionDeriveRouteOption {
  /** 1-based index the daemon resolves against the agent's `fallback_chain`. */
  route: number;
  label: string;
}

const FINGERPRINT_SUFFIX_LENGTH = 4;

/**
 * `Route {n} · {provider} · {model}` per declared route. Two routes that would
 * read the same differ by account, so each such route that names an account
 * gains a short fingerprint suffix; the raw command is never shown.
 */
export function sessionDeriveRouteOptions(
  routes: readonly SessionDeriveRoute[] | null | undefined
): SessionDeriveRouteOption[] {
  if (!routes || routes.length === 0) return [];
  const identities = routes.map(route => `${route.provider.trim()} · ${route.model.trim()}`);
  return routes.map((route, index) => {
    const identity = identities[index] ?? "";
    const collides = identities.some(
      (other, otherIndex) => otherIndex !== index && other === identity
    );
    const fingerprint = (route.command_fingerprint?.trim() ?? "").replace(/^sha256:/, "");
    const suffix =
      collides && fingerprint !== ""
        ? ` · ${fingerprint.slice(0, FINGERPRINT_SUFFIX_LENGTH)}…`
        : "";
    return { route: index + 1, label: `Route ${index + 1} · ${identity}${suffix}` };
  });
}
