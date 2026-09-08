import type { AgentEventPayload, ProviderErrorDiagnosticPayload } from "../types";

export type ProviderErrorCode = "provider_auth_required" | "provider_rate_limited";
/** Next actions the daemon emits; anything else renders the neutral `inspect` step. */
export type ProviderErrorNextAction = "login" | "bind_secret" | "inspect" | "retry";

export interface ProviderErrorView {
  code: ProviderErrorCode;
  provider: string;
  nextAction: ProviderErrorNextAction;
  occurrenceCount: number;
  firstSeenAt: string | null;
}

const CODES: ReadonlySet<string> = new Set<ProviderErrorCode>([
  "provider_auth_required",
  "provider_rate_limited",
]);
const NEXT_ACTIONS: ReadonlySet<string> = new Set<ProviderErrorNextAction>([
  "login",
  "bind_secret",
  "inspect",
  "retry",
]);

function knownDiagnostic(
  event: AgentEventPayload
): (ProviderErrorDiagnosticPayload & { code: ProviderErrorCode }) | null {
  const diagnostic = event.provider_error;
  if (!diagnostic || typeof diagnostic !== "object" || !CODES.has(diagnostic.code)) {
    return null;
  }
  return { ...diagnostic, code: diagnostic.code as ProviderErrorCode };
}

export function providerErrorView(event: AgentEventPayload): ProviderErrorView | null {
  const diagnostic = knownDiagnostic(event);
  if (!diagnostic) {
    return null;
  }
  const count = diagnostic.occurrence_count;
  const firstSeen = diagnostic.first_seen_at;
  return {
    code: diagnostic.code,
    provider: diagnostic.provider?.trim() || "The provider",
    nextAction: NEXT_ACTIONS.has(diagnostic.next_action)
      ? (diagnostic.next_action as ProviderErrorNextAction)
      : "inspect",
    occurrenceCount: Number.isFinite(count) && count >= 1 ? Math.floor(count) : 1,
    firstSeenAt:
      typeof firstSeen === "string" && Number.isFinite(Date.parse(firstSeen)) ? firstSeen : null,
  };
}

export function isProviderErrorEvent(event: AgentEventPayload): boolean {
  return event.type === "error" && knownDiagnostic(event) !== null;
}
