import type {
  SignInNoticeFixture,
  SignInReauthReason,
} from "@/systems/computers/mocks/remote-computers-fixtures-sign-in";

import type { AccountStatusPayload } from "../adapters/account-api";

export type ExtendedSignInReauthReason = SignInReauthReason | "account_suspended";

export type ExtendedSignInNoticeKind =
  | SignInNoticeFixture["kind"]
  | "rate_limited"
  | "linked_other_account"
  | "suspended";

export interface ResolvedSignInNotice {
  kind: ExtendedSignInNoticeKind;
  message: string;
  retryable?: boolean;
  showStatusLink?: boolean;
  showUnlinkAction?: boolean;
  showSupportLink?: boolean;
}

export interface ResolvedSignInView {
  title: string;
  subtitle?: string;
  notice?: ResolvedSignInNotice;
  buttonsDisabled: boolean;
  tokenRevoked: boolean;
  suspended: boolean;
}

const REAUTH_REASON_MESSAGES: Record<SignInReauthReason, string> = {
  device_revoked: "This computer was removed from your account. Sign in again.",
  account_deleted: "Your account was deleted.",
  session_invalid: "Your session ended. Sign in again.",
  computer_unlinked: "This computer was unlinked from your account. Sign in to keep using it.",
  token_revoked: "This account token was revoked.",
};

/** True when the installation sign-in gate blocks normal app routes and requires U0. */
export function isAccountGateBlocked(status: AccountStatusPayload | undefined): boolean {
  if (!status) return false;
  return status.gate === "enforced" && !status.signed_in && status.state !== "signed_in";
}

/** True when the daemon is running inside a policy or offline sign-in grace window. */
export function isAccountGraceActive(status: AccountStatusPayload | undefined): boolean {
  if (!status) return false;
  return status.gate === "grace" && !status.signed_in && status.grace != null;
}

/** Sanitizes a post-sign-in redirect target to a safe in-app relative URL. */
export function sanitizeSignInRedirect(raw: unknown): string | undefined {
  if (typeof raw !== "string") return undefined;
  const trimmed = raw.trim();
  if (!trimmed.startsWith("/") || trimmed.startsWith("//")) return undefined;
  if (trimmed === "/sign-in" || trimmed.startsWith("/sign-in?")) return undefined;
  return trimmed;
}

/** Formats the `rate_limited` notice copy for the given remaining seconds. */
export function formatRateLimitedMessage(remainingSeconds: number): string {
  const minutes = Math.max(1, Math.ceil(remainingSeconds / 60));
  return minutes === 1
    ? "Too many sign-in attempts. Try again in 1 minute."
    : `Too many sign-in attempts. Try again in ${minutes} minutes.`;
}

/** Maps a sign-in API error or SSE failure code into a U0 notice. */
export function noticeFromSignInError(input: {
  code?: string;
  message?: string;
  retryAfterSeconds?: number;
}): ResolvedSignInNotice {
  const code = input.code?.trim();
  switch (code) {
    case "access_denied":
    case "cancelled":
    case "canceled":
      return { kind: "cancelled", message: "Sign-in was cancelled." };
    case "account_login_expired":
    case "expired_token":
      return { kind: "timed_out", message: "Sign-in timed out. Try again." };
    case "account_cloud_unreachable":
    case "cloud_unavailable":
      return {
        kind: "cloud_unreachable",
        message: "Can't reach CompozyOS. Check your connection.",
        retryable: true,
        showStatusLink: true,
      };
    case "rate_limited":
      return {
        kind: "rate_limited",
        message: formatRateLimitedMessage(input.retryAfterSeconds ?? 60),
      };
    case "account_suspended":
      return {
        kind: "suspended",
        message: "This account is suspended.",
        showSupportLink: true,
      };
    case "account_deleting":
      return {
        kind: "account_deleting",
        message: "Your account is being deleted. Try again in a few minutes.",
      };
    case "installation_linked_to_other_account":
    case "link_account_mismatch":
      return {
        kind: "linked_other_account",
        message:
          input.message?.trim() ||
          "This computer is linked to another account for remote access. Unlink it before signing in with another account.",
        showUnlinkAction: true,
      };
    default:
      return {
        kind: "cancelled",
        message: input.message?.trim() || "Sign-in did not complete. Try again.",
      };
  }
}

/**
 * Resolves the U0 sign-in screen title, subtitle, notice, and control state from
 * `/api/account` status plus any active interactive sign-in feedback.
 */
export function resolveSignInView(
  status: AccountStatusPayload | undefined,
  options: {
    notice?: ResolvedSignInNotice | null;
    rateLimitRemainingSeconds?: number;
    cloudUnreachable?: boolean;
  } = {}
): ResolvedSignInView {
  const reason = status?.signed_out_reason ?? null;
  const isReauth = status?.state === "reauth_required";
  const isSuspended = reason === "account_suspended" || options.notice?.kind === "suspended";
  const isTokenRevoked = isReauth && reason === "token_revoked";
  const rateLimitRemaining = Math.max(0, options.rateLimitRemainingSeconds ?? 0);
  const isRateLimited = rateLimitRemaining > 0;

  let title = status?.pre_account_install
    ? "Sign in to keep using CompozyOS"
    : "Sign in to CompozyOS";
  let subtitle: string | undefined = status?.pre_account_install
    ? "Your sessions, projects and settings are still here."
    : "Run coding agents on all your computers.";

  let notice: ResolvedSignInNotice | undefined = options.notice ?? undefined;

  if (isSuspended) {
    title = "This account is suspended";
    subtitle = undefined;
    notice = {
      kind: "suspended",
      message: "This account is suspended.",
      showSupportLink: true,
    };
  } else if (isTokenRevoked) {
    title = "Sign in again";
    subtitle = undefined;
    notice = {
      kind: "token_revoked",
      message: REAUTH_REASON_MESSAGES.token_revoked,
    };
  } else if (isReauth && reason && reason in REAUTH_REASON_MESSAGES) {
    title = "Sign in again";
    subtitle = undefined;
    notice = notice ?? {
      kind: "reauth",
      message: REAUTH_REASON_MESSAGES[reason as SignInReauthReason],
    };
  } else if (isReauth) {
    title = "Sign in again";
    subtitle = undefined;
    notice = notice ?? {
      kind: "reauth",
      message: REAUTH_REASON_MESSAGES.session_invalid,
    };
  }

  if (options.cloudUnreachable && !isSuspended && !isTokenRevoked) {
    notice = {
      kind: "cloud_unreachable",
      message: "Can't reach CompozyOS. Check your connection.",
      retryable: true,
      showStatusLink: true,
    };
  }

  if (isRateLimited && !isSuspended && !isTokenRevoked) {
    notice = {
      kind: "rate_limited",
      message: formatRateLimitedMessage(rateLimitRemaining),
    };
  }

  const buttonsDisabled =
    Boolean(options.cloudUnreachable) || notice?.kind === "cloud_unreachable" || isRateLimited;

  return {
    title,
    subtitle,
    notice,
    buttonsDisabled,
    tokenRevoked: isTokenRevoked,
    suspended: isSuspended,
  };
}
