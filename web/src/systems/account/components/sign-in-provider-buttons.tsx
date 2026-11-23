import type { ReactNode } from "react";
import { GithubLogo, GoogleLogo } from "@compozy/ui/logos";

import { Button, cn, CopyIconButton, Surface } from "@compozy/ui";

import { BrandStage, type BrandStageVisual } from "@/systems/account/components/brand-stage";

import {
  SIGN_IN_PRIVACY_URL,
  SIGN_IN_REFUNDS_URL,
  SIGN_IN_TERMS_URL,
  type SignInProvider,
} from "@/systems/computers/mocks/remote-computers-fixtures-sign-in";

import { providerDisplayName } from "../lib/provider-display-name";

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
  const label = `Continue with ${providerDisplayName(provider)}`;

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
          "font-mono text-display-2xl font-semibold whitespace-nowrap tabular-nums select-all",
          struck ? "text-subtle line-through" : "text-fg-strong"
        )}
        style={{ letterSpacing: "0.22em" }}
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
  /** Brand Atmosphere on the art panel; sign-in pages use `swarm`, billing `cone`, success `converge`. */
  visual?: BrandStageVisual;
  /** One display line on the art panel (wide screens only). */
  statement?: ReactNode;
  onOpenLegalLink?: (kind: CloudLegalLinkKind) => void;
  dataSlot?: string;
  dataCloudKind?: string;
  className?: string;
}

/**
 * Shared chrome for `cloud.compozy.com` server-rendered pages: the brand art panel (wordmark,
 * Atmosphere visual, one statement) beside a centered column with an optional context pill, a
 * single card surface, an optional note and the legal footer.
 */
export function CloudPageChrome({
  children,
  headerAccessory,
  belowCard,
  maxWidthClass = "max-w-[440px]",
  cardClassName,
  visual = "swarm",
  statement = "Your agents, on all your computers.",
  onOpenLegalLink,
  dataSlot = "cloud-page",
  dataCloudKind,
  className,
}: CloudPageChromeProps) {
  return (
    <BrandStage
      className={className}
      data-cloud-kind={dataCloudKind}
      dataSlot={dataSlot}
      statement={statement}
      visual={visual}
    >
      <div className={cn("flex w-full flex-col items-center gap-5", maxWidthClass)}>
        {headerAccessory}

        <Surface
          className={cn("flex w-full flex-col gap-5 p-5 sm:p-7", cardClassName)}
          variant="card"
        >
          {children}
        </Surface>

        {belowCard}

        <CloudPageFooter className="mt-1" onOpenLegalLink={onOpenLegalLink} />
      </div>
    </BrandStage>
  );
}
