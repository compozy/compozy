/**
 * Sign-in (U0) and `cloud.compozy.com` browser page fixtures for Remote Computers.
 *
 * Agrees with `remote-computers-fixtures.ts` (`OWNER_EMAIL`, `LOCAL_COMPUTER`).
 * Pure data and types only; safe under `@compozy/proto` network guards.
 */
import { LOCAL_COMPUTER, OWNER_EMAIL } from "./remote-computers-fixtures";

export type SignInProvider = "google" | "github";

export type SignInReauthReason =
  | "device_revoked"
  | "account_deleted"
  | "session_invalid"
  | "computer_unlinked"
  | "token_revoked";

export type SignInNoticeKind =
  | "cancelled"
  | "timed_out"
  | "cloud_unreachable"
  | "reauth"
  | "account_deleting"
  | "token_revoked";

export interface SignInNoticeFixture {
  kind: SignInNoticeKind;
  message: string;
  retryable?: boolean;
}

export interface SignInWaitingAttemptFixture {
  attemptId: string;
  provider: SignInProvider;
  userCode: string;
  verificationUri: string;
  verificationUriComplete: string;
}

export interface SignInScreenPreset {
  title: string;
  subtitle?: string;
  notice?: SignInNoticeFixture;
  waiting?: SignInWaitingAttemptFixture;
  buttonsDisabled?: boolean;
  tokenRevoked?: boolean;
  cliHintCommand?: string;
  initialMarketingOptIn?: boolean;
}

export type CloudPageKind =
  | "activate"
  | "signed_in"
  | "email_unverified"
  | "code_used"
  | "unsubscribe";

export interface CloudPagePreset {
  kind: CloudPageKind;
  browserAddress: string;
  installationName: string;
  provider: SignInProvider;
  userCode: string;
  accountEmail: string;
  initialUnsubscribed?: boolean;
}

export const SIGN_IN_INSTALLATION_NAME = `${LOCAL_COMPUTER.name} (macOS)`;
export const SIGN_IN_PRIMARY_CODE = "WDJB-MJHT";
export const SIGN_IN_SECONDARY_CODE = "QFTX-PLRN";
export const SIGN_IN_OWNER_EMAIL = OWNER_EMAIL;
export const SIGN_IN_CLI_LOGIN_COMMAND = "compozy account login";
export const SIGN_IN_TERMS_URL = "https://compozy.com/terms";
export const SIGN_IN_PRIVACY_URL = "https://compozy.com/privacy";

export const SIGN_IN_REAUTH_MESSAGES: Record<SignInReauthReason, string> = {
  device_revoked: "This computer was removed from your account.",
  account_deleted: "Your account was deleted.",
  session_invalid: "Your session ended.",
  computer_unlinked: "This computer was unlinked from your account.",
  token_revoked: "This account token was revoked.",
};

export function buildWaitingAttempt(provider: SignInProvider): SignInWaitingAttemptFixture {
  const userCode = provider === "google" ? SIGN_IN_PRIMARY_CODE : SIGN_IN_SECONDARY_CODE;
  return {
    attemptId: provider === "google" ? "att_01k9s01google" : "att_01k9s02github",
    provider,
    userCode,
    verificationUri: "https://cloud.compozy.com/activate",
    verificationUriComplete: `https://cloud.compozy.com/activate?user_code=${userCode}&provider=${provider}`,
  };
}

export const SIGN_IN_WAITING_GOOGLE = buildWaitingAttempt("google");

export const SIGN_IN_DESKTOP_PRESETS: Record<string, SignInScreenPreset> = {
  "first-run": {
    title: "Sign in to Compozy",
    subtitle: "Run coding agents on all your computers.",
  },
  "existing-install": {
    title: "Sign in to keep using Compozy",
    subtitle: "Your sessions, projects and settings are still here.",
  },
  "waiting-browser": {
    title: "Sign in to Compozy",
    subtitle: "Run coding agents on all your computers.",
    waiting: SIGN_IN_WAITING_GOOGLE,
  },
  cancelled: {
    title: "Sign in to Compozy",
    subtitle: "Run coding agents on all your computers.",
    notice: {
      kind: "cancelled",
      message: "Sign-in was cancelled.",
    },
  },
  "cloud-unreachable": {
    title: "Sign in to Compozy",
    subtitle: "Run coding agents on all your computers.",
    buttonsDisabled: true,
    notice: {
      kind: "cloud_unreachable",
      message: "Can't reach Compozy. Check your connection.",
      retryable: true,
    },
  },
  "reauth-device-removed": {
    title: "Sign in again",
    notice: {
      kind: "reauth",
      message: SIGN_IN_REAUTH_MESSAGES.device_revoked,
    },
  },
  "timed-out": {
    title: "Sign in to Compozy",
    subtitle: "Run coding agents on all your computers.",
    notice: {
      kind: "timed_out",
      message: "Sign-in timed out. Try again.",
    },
  },
  "account-token-revoked": {
    title: "Sign in again",
    tokenRevoked: true,
    cliHintCommand: SIGN_IN_CLI_LOGIN_COMMAND,
    notice: {
      kind: "token_revoked",
      message: SIGN_IN_REAUTH_MESSAGES.token_revoked,
    },
  },
  "account-deleting": {
    title: "Sign in to Compozy",
    subtitle: "Run coding agents on all your computers.",
    notice: {
      kind: "account_deleting",
      message: "Your account is being deleted. Try again in a few minutes.",
    },
  },
};

export const SIGN_IN_CLOUD_PRESETS: Record<string, CloudPagePreset> = {
  "cloud-activate": {
    kind: "activate",
    browserAddress: `cloud.compozy.com/activate?user_code=${SIGN_IN_PRIMARY_CODE}&provider=google`,
    installationName: SIGN_IN_INSTALLATION_NAME,
    provider: "google",
    userCode: SIGN_IN_PRIMARY_CODE,
    accountEmail: OWNER_EMAIL,
  },
  "cloud-signed-in": {
    kind: "signed_in",
    browserAddress: "cloud.compozy.com/activate/done",
    installationName: SIGN_IN_INSTALLATION_NAME,
    provider: "google",
    userCode: SIGN_IN_PRIMARY_CODE,
    accountEmail: OWNER_EMAIL,
  },
  "cloud-email-unverified": {
    kind: "email_unverified",
    browserAddress: "cloud.compozy.com/activate/error",
    installationName: SIGN_IN_INSTALLATION_NAME,
    provider: "github",
    userCode: SIGN_IN_SECONDARY_CODE,
    accountEmail: OWNER_EMAIL,
  },
  "cloud-code-used": {
    kind: "code_used",
    browserAddress: "cloud.compozy.com/activate",
    installationName: SIGN_IN_INSTALLATION_NAME,
    provider: "google",
    userCode: SIGN_IN_PRIMARY_CODE,
    accountEmail: OWNER_EMAIL,
  },
  "cloud-unsubscribe": {
    kind: "unsubscribe",
    browserAddress: "cloud.compozy.com/email/unsubscribe",
    installationName: SIGN_IN_INSTALLATION_NAME,
    provider: "google",
    userCode: SIGN_IN_PRIMARY_CODE,
    accountEmail: OWNER_EMAIL,
    initialUnsubscribed: false,
  },
};
