import { useId } from "react";
import {
  CircleAlert,
  ClockAlert,
  ExternalLink,
  KeyRound,
  ShieldOff,
  WifiOff,
} from "@compozy/ui/icons";

import {
  Alert,
  AlertActions,
  AlertDescription,
  Button,
  Checkbox,
  cn,
  CodeBlock,
  Logo,
  Spinner,
  Surface,
} from "@compozy/ui";

import {
  SIGN_IN_CLI_LOGIN_COMMAND,
  SIGN_IN_PRIVACY_URL,
  SIGN_IN_TERMS_URL,
  type SignInNoticeFixture,
  type SignInNoticeKind,
  type SignInProvider,
  type SignInWaitingAttemptFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures-sign-in";

import {
  providerDisplayName,
  SignInProviderButton,
  SignInVerificationCode,
} from "./sign-in-provider-buttons";

const PROVIDERS: readonly SignInProvider[] = ["google", "github"];

export interface SignInScreenProps {
  title: string;
  subtitle?: string;
  notice?: SignInNoticeFixture;
  waiting?: SignInWaitingAttemptFixture;
  buttonsDisabled?: boolean;
  /** Account-token installation whose token was revoked: shows CLI hint without provider buttons. */
  tokenRevoked?: boolean;
  cliHintCommand?: string;
  marketingOptIn?: boolean;
  autoFocusFirstProvider?: boolean;
  onMarketingOptInChange?: (checked: boolean) => void;
  onSelectProvider?: (provider: SignInProvider) => void;
  onReopenBrowser?: (attempt: SignInWaitingAttemptFixture) => void;
  onCancelWaiting?: () => void;
  onRetryConnection?: () => void;
  onOpenLegalLink?: (kind: "terms" | "privacy") => void;
  className?: string;
}

function noticeVariant(kind: SignInNoticeKind): "default" | "warning" {
  return kind === "cancelled" ? "default" : "warning";
}

function NoticeIcon({ kind }: { kind: SignInNoticeKind }) {
  switch (kind) {
    case "cloud_unreachable":
      return <WifiOff aria-hidden="true" />;
    case "reauth":
      return <ShieldOff aria-hidden="true" />;
    case "token_revoked":
      return <KeyRound aria-hidden="true" />;
    case "timed_out":
    case "account_deleting":
      return <ClockAlert aria-hidden="true" />;
    case "cancelled":
      return <CircleAlert aria-hidden="true" />;
  }
}

interface WaitingPanelProps {
  attempt: SignInWaitingAttemptFixture;
  onReopenBrowser?: (attempt: SignInWaitingAttemptFixture) => void;
  onCancelWaiting?: () => void;
}

function SignInWaitingPanel({ attempt, onReopenBrowser, onCancelWaiting }: WaitingPanelProps) {
  return (
    <Surface className="flex flex-col gap-3.5 p-4" data-slot="sign-in-waiting-panel" variant="card">
      <div className="flex min-w-0 items-center gap-2.5">
        <Spinner aria-hidden="true" className="size-4 shrink-0 text-fg-2" />
        <span className="truncate text-item-title font-medium text-fg-strong">
          Continue in your browser with {providerDisplayName(attempt.provider)}
        </span>
      </div>

      <SignInVerificationCode code={attempt.userCode} copyable />

      <p className="text-center text-meta text-muted-foreground">
        Check that your browser shows the same code.
      </p>

      <div className="grid grid-cols-2 gap-2 pt-0.5">
        <Button
          className="w-full"
          onClick={() => onReopenBrowser?.(attempt)}
          size="default"
          type="button"
          variant="secondary"
        >
          <span>Reopen browser</span>
          <ExternalLink aria-hidden="true" className="size-3.5" />
        </Button>
        <Button
          className="w-full"
          onClick={onCancelWaiting}
          size="default"
          type="button"
          variant="secondary"
        >
          Cancel
        </Button>
      </div>
    </Surface>
  );
}

/**
 * Full-window sign-in gate (`_uiux.md` U0, ADR-011).
 *
 * Centered single column on the app window canvas (no rail, no sidebar, no palette).
 * Stays locked to `max-w-[380px]` at laptop (1396 px) and wide (2084 px+) widths.
 */
export function SignInScreen({
  title,
  subtitle,
  notice,
  waiting,
  buttonsDisabled = false,
  tokenRevoked = false,
  cliHintCommand = SIGN_IN_CLI_LOGIN_COMMAND,
  marketingOptIn = false,
  autoFocusFirstProvider = false,
  onMarketingOptInChange,
  onSelectProvider,
  onReopenBrowser,
  onCancelWaiting,
  onRetryConnection,
  onOpenLegalLink,
  className,
}: SignInScreenProps) {
  const consentId = useId();

  return (
    <main
      className={cn(
        "flex min-h-0 flex-1 flex-col items-center justify-center px-6 pt-4 pb-12 text-fg",
        className
      )}
      data-slot="sign-in-screen"
    >
      <p aria-live="polite" className="sr-only" role="status">
        {waiting ? "Waiting for sign-in in your browser" : ""}
      </p>

      <div className="flex w-full max-w-[380px] flex-col">
        <div className="flex flex-col items-center text-center">
          <Logo className="size-11" decorative variant="symbol" />
          <h1 className="mt-6 text-display-2xl font-medium tracking-detail-h1 text-fg-strong text-balance">
            {title}
          </h1>
          {subtitle ? (
            <p className="mt-2 max-w-[34ch] text-body text-muted-foreground text-pretty">
              {subtitle}
            </p>
          ) : null}
        </div>

        {notice ? (
          <Alert className="mt-6" variant={noticeVariant(notice.kind)}>
            <NoticeIcon kind={notice.kind} />
            <AlertDescription className="text-fg">{notice.message}</AlertDescription>
            {notice.retryable ? (
              <AlertActions className="justify-start pt-0.5">
                <Button onClick={onRetryConnection} size="sm" type="button" variant="secondary">
                  Try again
                </Button>
              </AlertActions>
            ) : null}
          </Alert>
        ) : null}

        {tokenRevoked ? (
          <Surface
            className="mt-6 flex flex-col gap-3 p-4"
            data-slot="sign-in-token-hint"
            variant="card"
          >
            <p className="text-small-body leading-relaxed text-muted-foreground">
              Restart Compozy with a new{" "}
              <code className="font-mono text-meta text-fg">COMPOZY_ACCOUNT_TOKEN</code>, or sign in
              interactively from your terminal:
            </p>
            <CodeBlock
              code={cliHintCommand}
              copyable
              density="compact"
              language="bash"
              showPrompt
            />
          </Surface>
        ) : (
          <>
            <div className="mt-7 flex flex-col gap-2.5" data-slot="sign-in-actions">
              {PROVIDERS.map((provider, index) => {
                if (waiting && waiting.provider === provider) {
                  return (
                    <SignInWaitingPanel
                      attempt={waiting}
                      key={provider}
                      onCancelWaiting={onCancelWaiting}
                      onReopenBrowser={onReopenBrowser}
                    />
                  );
                }

                return (
                  <SignInProviderButton
                    autoFocus={autoFocusFirstProvider && index === 0 && !waiting}
                    disabled={buttonsDisabled}
                    key={provider}
                    onClick={() => onSelectProvider?.(provider)}
                    provider={provider}
                  />
                );
              })}
            </div>

            <div className="mt-6 flex justify-center">
              <label
                className={cn(
                  "inline-flex items-center gap-2.5 text-small-body text-muted-foreground select-none",
                  buttonsDisabled ? "cursor-not-allowed opacity-60" : "cursor-pointer hover:text-fg"
                )}
                htmlFor={consentId}
              >
                <Checkbox
                  checked={marketingOptIn}
                  disabled={buttonsDisabled}
                  id={consentId}
                  onCheckedChange={next => onMarketingOptInChange?.(Boolean(next))}
                />
                <span>Email me product news and offers</span>
              </label>
            </div>

            <p className="mt-6 text-center text-meta leading-relaxed text-subtle">
              By continuing, you agree to the{" "}
              <a
                className="text-fg-2 underline decoration-line-strong underline-offset-3 transition-colors hover:text-fg"
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
              </a>{" "}
              and{" "}
              <a
                className="text-fg-2 underline decoration-line-strong underline-offset-3 transition-colors hover:text-fg"
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
              .
            </p>
          </>
        )}
      </div>
    </main>
  );
}
