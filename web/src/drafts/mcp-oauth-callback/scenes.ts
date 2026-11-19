/**
 * Fixtures for the MCP OAuth callback page: one scene per outcome the daemon can tell apart at
 * `GET /api/mcp/oauth/callback`, shipped as `internal/api/mcpoauthpage` (Go template with
 * embedded assets, classification in `internal/api/core`); every field here maps to data it holds
 * (`mcpauth.Status` on success, the classified error on failure) or to a sentinel it can match
 * (`ErrLoginSessionExpired`, `ErrLoginSessionNotFound`, `ErrLoginSessionStale`, the provider's
 * `error=` query value, the 503 path).
 */

export type CallbackOutcome =
  | "connected"
  | "declined"
  | "expired"
  | "changed"
  | "unavailable"
  | "failed";

export interface CallbackServer {
  name: string;
  /** Host of `remote_url`, the server the person signed in to. */
  host: string;
}

export interface CallbackScene {
  outcome: CallbackOutcome;
  /** Absent when the OAuth `state` no longer resolves to a sign-in (expired, already used). */
  server?: CallbackServer;
  title: string;
  detail: string;
  /** Where the connection applies: a project or every project. */
  availability?: string;
  scopes?: string[];
  /** When the token is refreshed by CompozyOS on its own, or when it lapses. */
  renewal?: string;
  /** Machine-readable reason, shown in mono under the message. */
  code?: string;
  /** Raw diagnostic, only for the unclassified failure. */
  diagnostic?: string;
  /** Time the page was rendered, for the status line. */
  at: string;
}

const LINEAR: CallbackServer = { name: "Linear", host: "mcp.linear.app" };

export const SCENES: Record<string, CallbackScene> = {
  connected: {
    outcome: "connected",
    server: LINEAR,
    title: "Linear is connected",
    detail: "Agents in CompozyOS can use Linear’s tools now. You can close this tab.",
    availability: "Project · compozy",
    scopes: ["read", "write", "issues:create"],
    renewal: "Renews on its own",
    at: "8:41 AM",
  },
  "connected-all-projects": {
    outcome: "connected",
    server: { name: "GitHub", host: "api.githubcopilot.com" },
    title: "GitHub is connected",
    detail: "Every project in CompozyOS can use GitHub’s tools now. You can close this tab.",
    availability: "All projects",
    renewal: "Expires in 8 hours",
    at: "8:41 AM",
  },
  "connected-long": {
    outcome: "connected",
    server: {
      name: "acme-internal-knowledge-base",
      host: "kb-gateway.eu-west-1.internal.acme-corp.dev",
    },
    title: "acme-internal-knowledge-base is connected",
    detail:
      "Agents in CompozyOS can use acme-internal-knowledge-base’s tools now. You can close this tab.",
    availability: "Project · platform-monorepo-services",
    scopes: [
      "openid",
      "profile",
      "offline_access",
      "kb:read",
      "kb:search",
      "kb:write",
      "attachments:read",
      "admin:audit-log:read",
    ],
    renewal: "Renews on its own",
    at: "8:41 AM",
  },
  declined: {
    outcome: "declined",
    server: LINEAR,
    title: "Linear didn’t grant access",
    detail:
      "The sign-in was canceled or declined on Linear. Nothing changed in CompozyOS. Start again from the server’s settings when you’re ready.",
    code: "access_denied",
    at: "8:41 AM",
  },
  expired: {
    outcome: "expired",
    title: "This sign-in link has expired",
    detail:
      "Sign-in links last 5 minutes and work only once. Start a new sign-in from the server’s settings in CompozyOS.",
    code: "login_session_expired",
    at: "8:47 AM",
  },
  changed: {
    outcome: "changed",
    server: LINEAR,
    title: "Linear changed while you signed in",
    detail:
      "The server’s settings were edited during sign-in, so this one was set aside to keep the new settings. Sign in again to finish.",
    code: "login_session_stale",
    at: "8:41 AM",
  },
  unavailable: {
    outcome: "unavailable",
    title: "CompozyOS couldn’t finish signing in",
    detail:
      "The runtime wasn’t ready to take this sign-in. Open CompozyOS, wait until it has started, and sign in again.",
    code: "HTTP 503 · settings_unavailable",
    at: "8:41 AM",
  },
  failed: {
    outcome: "failed",
    server: LINEAR,
    title: "Sign-in didn’t finish",
    detail:
      "Linear answered, but CompozyOS couldn’t accept the answer. Try again, or use manual sign-in from the server’s settings.",
    code: "invalid_exchange",
    diagnostic:
      "settings: complete MCP auth callback: mcp auth: token endpoint returned 400: invalid_grant (code verifier does not match challenge)",
    at: "8:41 AM",
  },
};
