import { AlertTriangle, CheckCircle2, CircleAlert, MailIcon } from "@compozy/ui/icons";

import { Button, cn, Logo, Surface } from "@compozy/ui";

import {
  SIGN_IN_PRIVACY_URL,
  SIGN_IN_TERMS_URL,
  type CloudPageKind,
  type SignInProvider,
} from "@/systems/computers/mocks/remote-computers-fixtures-sign-in";

import {
  providerDisplayName,
  SignInProviderButton,
  SignInVerificationCode,
} from "./sign-in-provider-buttons";

export interface SignInCloudPageProps {
  kind: CloudPageKind;
  installationName: string;
  provider: SignInProvider;
  userCode: string;
  accountEmail: string;
  /** When `kind === "unsubscribe"`, toggles between confirmation (GET) and completed (POST). */
  unsubscribed?: boolean;
  onContinueProvider?: (provider: SignInProvider) => void;
  onConfirmUnsubscribe?: () => void;
  onOpenLegalLink?: (kind: "terms" | "privacy") => void;
  className?: string;
}

/**
 * Browser-hosted pages on `cloud.compozy.com` (`_uiux.md` U0, ADR-011):
 * - `/activate`: code confirmation + provider continuation
 * - `/activate/done`: sign-in completed, return to Compozy
 * - `/activate/error`: unverified provider email
 * - `/activate` (code used): already-redeemed device code
 * - `/email/unsubscribe`: confirmation page (GET shows button; POST applies change)
 */
export function SignInCloudPage({
  kind,
  installationName,
  provider,
  userCode,
  accountEmail,
  unsubscribed = false,
  onContinueProvider,
  onConfirmUnsubscribe,
  onOpenLegalLink,
  className,
}: SignInCloudPageProps) {
  const providerLabel = providerDisplayName(provider);

  return (
    <main
      className={cn(
        "flex min-h-0 flex-1 flex-col items-center justify-center bg-canvas px-6 py-10 text-fg",
        className
      )}
      data-cloud-kind={kind}
      data-slot="sign-in-cloud-page"
    >
      <div className="flex w-full max-w-[420px] flex-col items-center gap-7">
        <Logo className="h-7" variant="logo" />

        <Surface className="flex w-full flex-col gap-5 p-7" variant="card">
          {kind === "activate" ? (
            <>
              <div className="flex flex-col items-center gap-1.5 text-center">
                <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong text-balance">
                  Sign in to Compozy
                </h1>
                <p className="text-small-body text-muted-foreground">
                  Signing in on <span className="font-medium text-fg">{installationName}</span>
                </p>
              </div>

              <SignInVerificationCode code={userCode} />

              <p className="text-center text-meta text-muted-foreground">
                Confirm this code matches what Compozy shows on your computer.
              </p>

              <SignInProviderButton
                onClick={() => onContinueProvider?.(provider)}
                primary
                provider={provider}
              />
            </>
          ) : null}

          {kind === "signed_in" ? (
            <div className="flex flex-col items-center gap-3.5 py-2 text-center">
              <span
                aria-hidden="true"
                className="inline-flex size-11 items-center justify-center rounded-lg bg-success-tint text-success"
              >
                <CheckCircle2 className="size-5" />
              </span>
              <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong text-balance">
                You&apos;re signed in. Return to Compozy.
              </h1>
              <p className="text-small-body leading-relaxed text-muted-foreground text-pretty">
                <span className="font-medium text-fg">{installationName}</span> is ready for{" "}
                <span className="font-medium text-fg">{accountEmail}</span>. You can close this tab.
              </p>
            </div>
          ) : null}

          {kind === "email_unverified" ? (
            <>
              <div className="flex flex-col items-center gap-3 text-center">
                <span
                  aria-hidden="true"
                  className="inline-flex size-11 items-center justify-center rounded-lg bg-warning-tint text-warning"
                >
                  <AlertTriangle className="size-5" />
                </span>
                <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong text-balance">
                  Email verification required
                </h1>
                <p className="text-small-body leading-relaxed text-muted-foreground text-pretty">
                  Your {providerLabel} account has no verified email. Verify one and try again.
                </p>
              </div>

              <SignInProviderButton
                onClick={() => onContinueProvider?.(provider)}
                provider={provider}
              />

              <p className="text-center text-meta text-subtle">
                Compozy is still waiting on {installationName}.
              </p>
            </>
          ) : null}

          {kind === "code_used" ? (
            <>
              <div className="flex flex-col items-center gap-3 text-center">
                <span
                  aria-hidden="true"
                  className="inline-flex size-11 items-center justify-center rounded-lg bg-surface-2 text-subtle"
                >
                  <CircleAlert className="size-5" />
                </span>
                <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong text-balance">
                  This code was already used.
                </h1>
                <p className="text-small-body leading-relaxed text-muted-foreground text-pretty">
                  Start a new sign-in from Compozy on{" "}
                  <span className="font-medium text-fg">{installationName}</span> to get a fresh
                  code.
                </p>
              </div>

              <SignInVerificationCode code={userCode} struck />
            </>
          ) : null}

          {kind === "unsubscribe" && !unsubscribed ? (
            <>
              <div className="flex flex-col items-center gap-3 text-center">
                <span
                  aria-hidden="true"
                  className="inline-flex size-11 items-center justify-center rounded-lg bg-surface-2 text-subtle"
                >
                  <MailIcon className="size-5" />
                </span>
                <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong text-balance">
                  Unsubscribe from news and offers?
                </h1>
                <p className="text-small-body leading-relaxed text-muted-foreground text-pretty">
                  You will stop receiving product news, tips and occasional offers at{" "}
                  <span className="font-medium text-fg">{accountEmail}</span>. Account and security
                  emails are unaffected.
                </p>
              </div>

              <Button
                className="w-full"
                onClick={onConfirmUnsubscribe}
                size="cta-lg"
                type="button"
                variant="default"
              >
                Unsubscribe
              </Button>
            </>
          ) : null}

          {kind === "unsubscribe" && unsubscribed ? (
            <div className="flex flex-col items-center gap-3.5 py-2 text-center">
              <span
                aria-hidden="true"
                className="inline-flex size-11 items-center justify-center rounded-lg bg-success-tint text-success"
              >
                <CheckCircle2 className="size-5" />
              </span>
              <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong text-balance">
                You&apos;re unsubscribed
              </h1>
              <p className="text-small-body leading-relaxed text-muted-foreground text-pretty">
                You won&apos;t get product news and offers anymore at{" "}
                <span className="font-medium text-fg">{accountEmail}</span>. You can turn this back
                on anytime in Settings → Account.
              </p>
            </div>
          ) : null}
        </Surface>

        <footer className="flex items-center justify-center gap-2.5 text-meta text-subtle">
          <span>cloud.compozy.com</span>
          <span aria-hidden="true">·</span>
          <a
            className="transition-colors hover:text-fg"
            href={SIGN_IN_TERMS_URL}
            onClick={event => {
              if (onOpenLegalLink) {
                event.preventDefault();
                onOpenLegalLink("terms");
              }
            }}
            rel="noreferrer"
            target="_blank"
          >
            Terms
          </a>
          <span aria-hidden="true">·</span>
          <a
            className="transition-colors hover:text-fg"
            href={SIGN_IN_PRIVACY_URL}
            onClick={event => {
              if (onOpenLegalLink) {
                event.preventDefault();
                onOpenLegalLink("privacy");
              }
            }}
            rel="noreferrer"
            target="_blank"
          >
            Privacy Policy
          </a>
        </footer>
      </div>
    </main>
  );
}
