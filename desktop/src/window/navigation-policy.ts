import { MACOS_ACCESSIBILITY_SETTINGS_URL } from "../shortcuts/accessibility";

export type NavigationDecision = "allow" | "external" | "deny";

function parsed(raw: string): URL | null {
  try {
    return new URL(raw);
  } catch {
    return null;
  }
}

export function safeExternalURL(raw: string): string | null {
  const target = parsed(raw);
  return target && isExternalURL(target) ? target.toString() : null;
}

function isExternalURL(target: URL): boolean {
  return (
    target.protocol === "http:" ||
    target.protocol === "https:" ||
    target.toString() === MACOS_ACCESSIBILITY_SETTINGS_URL
  );
}

export function classifyNavigation(raw: string, daemonOrigin: string): NavigationDecision {
  const target = parsed(raw);
  const daemon = parsed(daemonOrigin);
  if (!target || !daemon) return "deny";
  if (target.origin === daemon.origin) return "allow";
  return isExternalURL(target) ? "external" : "deny";
}
