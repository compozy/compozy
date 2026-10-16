import { MACOS_ACCESSIBILITY_SETTINGS_URL } from "../shortcuts/accessibility";

export type NavigationDecision = "allow" | "external" | "deny";

/** Parses navigation input without throwing on malformed URLs. */
function parsed(raw: string): URL | null {
  try {
    return new URL(raw);
  } catch {
    return null;
  }
}

/** Returns only URLs supported by the operating-system external opener. */
export function safeExternalURL(raw: string): string | null {
  const target = parsed(raw);
  return target && isExternalURL(target) ? target.toString() : null;
}

/** Limits external opens to web URLs and the exact accessibility settings link. */
function isExternalURL(target: URL): boolean {
  return (
    target.protocol === "http:" ||
    target.protocol === "https:" ||
    target.toString() === MACOS_ACCESSIBILITY_SETTINGS_URL
  );
}

/** Checks decoded server paths so encoded API URLs cannot bypass the document guard. */
function isAPIDocument(target: URL): boolean {
  try {
    const pathname = decodeURIComponent(target.pathname);
    return pathname === "/api" || pathname.startsWith("/api/");
  } catch {
    return true;
  }
}

/** Allows SPA navigation while blocking same-origin API documents from replacing the app. */
export function classifyNavigation(raw: string, daemonOrigin: string): NavigationDecision {
  const target = parsed(raw);
  const daemon = parsed(daemonOrigin);
  if (!target || !daemon) return "deny";
  if (target.origin === daemon.origin) {
    if (isAPIDocument(target)) return "deny";
    return "allow";
  }
  return isExternalURL(target) ? "external" : "deny";
}
