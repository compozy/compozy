/**
 * Human-readable copy for every remote-scope and remote-boundary rejection
 * returned by the controller bridge or a remote host (`_dx.md`, `_uiux.md`).
 *
 * Every rejection renders plain English rather than raw JSON or machine codes.
 */

export interface RemoteRejectionCopy {
  title: string;
  detail: string;
}

const REMOTE_SCOPE_LABELS: Readonly<Record<string, string>> = {
  "sessions:operate": "Operating sessions",
  "terminal:operate": "Terminals",
  "files:read": "Reading files",
  "files:write": "Writing files",
  "tasks:operate": "Operating tasks",
  "settings:write": "Changing settings",
  "remote:admin": "Remote administration",
};

const STATIC_REMOTE_REJECTIONS: Readonly<Record<string, RemoteRejectionCopy>> = {
  remote_entitlement_unavailable: {
    title: "Remote access paused",
    detail:
      "Your Pro plan ended. Running agents keep running — upgrade to Pro to reconnect.",
  },
  plan_required: {
    title: "Pro plan required",
    detail: "Upgrade to Pro in Settings → Account to use this feature.",
  },
  device_revoked: {
    title: "This device was revoked",
    detail: "Sign in again in Settings → Account to reconnect.",
  },
  device_pending_approval: {
    title: "Waiting for device approval",
    detail: "Approve this device from an admin device in Settings → Computers.",
  },
  device_not_approved: {
    title: "This device isn't approved",
    detail: "Approve this device from an admin device in Settings → Computers.",
  },
  route_not_remote: {
    title: "Not available remotely",
    detail: "This action can only run directly on that computer.",
  },
  remote_bridge_not_transitive: {
    title: "Nested remote access isn't allowed",
    detail: "Connect directly to that computer from your controller instead.",
  },
  remote_bridge_browser_denied: {
    title: "Not available from a browser",
    detail: "Open CompozyOS on a desktop or CLI controller to access other computers.",
  },
  claim_token_not_remote: {
    title: "Sign-in link must run locally",
    detail: "Complete this sign-in directly on that computer.",
  },
  tool_not_forwardable: {
    title: "Tool can't run remotely",
    detail: "This tool only runs on the computer hosting the session.",
  },
  tool_forward_denied: {
    title: "Remote tool blocked",
    detail: "This device doesn't have permission to run that tool on the remote computer.",
  },
  remote_caller_override_forbidden: {
    title: "Caller identity override rejected",
    detail: "Remote requests always run under your signed-in device identity.",
  },
  computer_not_owned: {
    title: "Computer doesn't belong to this account",
    detail: "Check that both computers are signed in to the same Compozy account.",
  },
};

export function isRemoteRejectionCode(code: string | null | undefined): boolean {
  if (!code) return false;
  return code === "remote_scope_denied" || Object.hasOwn(STATIC_REMOTE_REJECTIONS, code);
}

/** Extracts string `details` fields from a daemon/bridge error body, if present. */
export function apiErrorDetails(error: unknown): Readonly<Record<string, string>> | undefined {
  if (error == null || typeof error !== "object") return undefined;
  const candidate = Reflect.get(error, "details");
  if (candidate == null || typeof candidate !== "object") return undefined;
  const result: Record<string, string> = {};
  let hasEntry = false;
  for (const [key, value] of Object.entries(candidate as Record<string, unknown>)) {
    if (typeof value === "string" && value.trim() !== "") {
      result[key] = value.trim();
      hasEntry = true;
    }
  }
  return hasEntry ? Object.freeze(result) : undefined;
}

export function remoteRejectionCopy(
  code: string | null | undefined,
  details?: Readonly<Record<string, unknown>> | null
): RemoteRejectionCopy | null {
  if (!code) return null;
  if (code === "remote_scope_denied") {
    const rawScope = details && typeof details.scope === "string" ? details.scope.trim() : "";
    const label = rawScope ? REMOTE_SCOPE_LABELS[rawScope] : undefined;
    if (label) {
      return {
        title: `${label} ${label === "Terminals" ? "aren't" : "isn't"} allowed from this device`,
        detail: `Ask an admin device to enable ${rawScope} in Settings → Computers → Permissions.`,
      };
    }
    return {
      title: "Permission denied on this computer",
      detail: rawScope
        ? `Ask an admin device to enable ${rawScope} in Settings → Computers → Permissions.`
        : "Ask an admin device to enable this permission in Settings → Computers → Permissions.",
    };
  }
  return Object.hasOwn(STATIC_REMOTE_REJECTIONS, code)
    ? (STATIC_REMOTE_REJECTIONS[code] ?? null)
    : null;
}

export function remoteRejectionMessage(
  code: string | null | undefined,
  fallback: string,
  details?: Readonly<Record<string, unknown>> | null
): string {
  const copy = remoteRejectionCopy(code, details);
  if (!copy) return fallback;
  return copy.detail ? `${copy.title}. ${copy.detail}` : copy.title;
}
