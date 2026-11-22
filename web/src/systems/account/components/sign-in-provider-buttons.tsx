import { GithubLogo, GoogleLogo } from "@compozy/ui/logos";

import { Button, cn, CopyIconButton } from "@compozy/ui";

import type { SignInProvider } from "@/systems/computers/mocks/remote-computers-fixtures-sign-in";

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
