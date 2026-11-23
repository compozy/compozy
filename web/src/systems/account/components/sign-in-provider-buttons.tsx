import type { ReactNode } from "react";
import { GithubLogo, GoogleLogo } from "@compozy/ui/logos";

import { Button, cn, CopyIconButton, Logo, Surface } from "@compozy/ui";

import {
  SIGN_IN_PRIVACY_URL,
  SIGN_IN_REFUNDS_URL,
  SIGN_IN_TERMS_URL,
  type SignInProvider,
} from "@/systems/computers/mocks/remote-computers-fixtures-sign-in";

const PROVIDER_LABEL: Record<SignInProvider, string> = {
  google: "Google",
  github: "GitHub",
};

export function providerDisplayName(provider: SignInProvider): string {
  return PROVIDER_LABEL[provider];
}

export interface SignInProviderGlyphProps {
  provider: SignInProvider;
  className?: string;
}

export function SignInProviderGlyph({ provider, className }: SignInProviderGlyphProps) {
  if (provider === "google") {
    return <GoogleLogo aria-hidden="true" className={cn("size-4 shrink-0", className)} />;
  }
  return <GithubLogo aria-hidden="true" className={cn("size-4 shrink-0", className)} />;
}

export interface SignInProviderButtonProps {
  provider: SignInProvider;
  disabled?: boolean;
  /** Single primary CTA treatment (e.g. cloud activation page). Equal-weight outline by default. */
  primary?: boolean;
  autoFocus?: boolean;
  onClick?: () => void;
  className?: string;
}

/**
 * Full-width provider sign-in button with official brand logo from `@compozy/ui/logos`.
 */
export function SignInProviderButton({
  provider,
  disabled = false,
  primary = false,
  autoFocus = false,
  onClick,
  className,
}: SignInProviderButtonProps) {
  const label = `Continue with ${PROVIDER_LABEL[provider]}`;

  return (
    <Button
      autoFocus={autoFocus}
      className={cn(
        "w-full justify-center gap-2.5",
        !primary &&
          "border-line-strong bg-card text-fg-strong shadow-card hover:bg-surface-2 disabled:border-line-soft disabled:bg-surface-2 disabled:text-subtle disabled:shadow-none",
        className
      )}
      data-provider={provider}
      data-slot="sign-in-provider-button"
      disabled={disabled}
      onClick={onClick}
      size="cta-lg"
      type="button"
      variant={primary ? "default" : "outline"}
    >
      <SignInProviderGlyph
        className={cn("size-4.5", disabled && "opacity-50")}
        provider={provider}
      />
      <span>{label}</span>
    </Button>
  );
}

export interface SignInVerificationCodeProps {
  code: string;
  copyable?: boolean;
  struck?: boolean;
  className?: string;
}

/**
 * Large single-line monospace device verification code (`WDJB-MJHT`) on a sunken inset plate.
 */
export function SignInVerificationCode({
  code,
  copyable = false,
  struck = false,
  className,
}: SignInVerificationCodeProps) {
  return (
    <div
      className={cn(
        "relative flex items-center justify-center rounded-md bg-sunken px-4 py-3.5",
        className
      )}
      data-slot="sign-in-verification-code"
    >
      <span
        className={cn(
          "font-mono text-display-2xl font-semibold tracking-[0.22em] whitespace-nowrap tabular-nums select-all",
          struck ? "text-subtle line-through" : "text-fg-strong"
        )}
      >
        {code}
      </span>
      {copyable ? (
        <CopyIconButton
          className="absolute right-2 text-subtle hover:text-fg"
          copiedLabel="Code copied"
          copiedToastLabel="Code copied"
          copyLabel="Copy verification code"
          size="icon-sm"
          value={code}
        />
      ) : null}
    </div>
  );
}

export type CloudLegalLinkKind = "terms" | "privacy" | "refunds";

export interface CloudPageFooterProps {
  onOpenLegalLink?: (kind: CloudLegalLinkKind) => void;
  className?: string;
}

const CLOUD_LEGAL_LINKS: ReadonlyArray<{
  kind: CloudLegalLinkKind;
  label: string;
  href: string;
}> = [
  { kind: "terms", label: "Terms", href: SIGN_IN_TERMS_URL },
  { kind: "privacy", label: "Privacy", href: SIGN_IN_PRIVACY_URL },
  { kind: "refunds", label: "Refunds", href: SIGN_IN_REFUNDS_URL },
];

/**
 * Shared legal footer rendered on every `cloud.compozy.com` browser page (`_uiux.md` U0, U12).
 */
export function CloudPageFooter({ onOpenLegalLink, className }: CloudPageFooterProps) {
  return (
    <footer
      className={cn(
        "flex flex-wrap items-center justify-center gap-x-2.5 gap-y-1 text-meta text-subtle",
        className
      )}
      data-slot="cloud-page-footer"
    >
      <span>cloud.compozy.com</span>
      {CLOUD_LEGAL_LINKS.map(link => (
        <span className="inline-flex items-center gap-2.5" key={link.kind}>
          <span aria-hidden="true">·</span>
          <a
            className="transition-colors hover:text-fg"
            href={link.href}
            onClick={event => {
              if (onOpenLegalLink) {
                event.preventDefault();
                onOpenLegalLink(link.kind);
              }
            }}
            rel="noreferrer"
            target="_blank"
          >
            {link.label}
          </a>
        </span>
      ))}
    </footer>
  );
}

export interface CloudPageChromeProps {
  children: ReactNode;
  headerAccessory?: ReactNode;
  belowCard?: ReactNode;
  maxWidthClass?: string;
  cardClassName?: string;
  onOpenLegalLink?: (kind: CloudLegalLinkKind) => void;
  dataSlot?: string;
  dataCloudKind?: string;
  className?: string;
}

/**
 * Shared centered column chrome for `cloud.compozy.com` server-rendered pages:
 * Compozy logo, optional context pill, a single card surface, optional note, and legal footer.
 */
export function CloudPageChrome({
  children,
  headerAccessory,
  belowCard,
  maxWidthClass = "max-w-[440px]",
  cardClassName,
  onOpenLegalLink,
  dataSlot = "cloud-page",
  dataCloudKind,
  className,
}: CloudPageChromeProps) {
  return (
    <main
      className={cn(
        "flex min-h-0 flex-1 flex-col items-center justify-center bg-canvas px-4 py-8 text-fg sm:px-6 sm:py-10",
        className
      )}
      data-cloud-kind={dataCloudKind}
      data-slot={dataSlot}
    >
      <div className={cn("flex w-full flex-col items-center gap-6", maxWidthClass)}>
        <div className="flex flex-col items-center gap-2.5">
          <Logo className="h-7" variant="logo" />
          {headerAccessory}
        </div>

        <Surface className={cn("flex w-full flex-col gap-5 p-5 sm:p-7", cardClassName)} variant="card">
          {children}
        </Surface>

        {belowCard}

        <CloudPageFooter onOpenLegalLink={onOpenLegalLink} />
      </div>
    </main>
  );
}
