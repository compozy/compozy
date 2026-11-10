/** Marketing type ramp, links and calls to action on the shipped tokens (ported from the landing draft). */
import { Button, buttonVariants, cn } from "@compozy/ui";
import Link from "next/link";
import type { ComponentType, ReactNode, SVGProps } from "react";

import { isExternalHref } from "@/components/site/links";

export {
  Camera,
  InstallCommand,
  Plate,
  ProductWindow,
  Shot,
} from "@/components/site/primitives-client";

export type Icon = ComponentType<SVGProps<SVGSVGElement>>;

export function LandingEyebrow({
  children,
  icon: Glyph,
  className,
}: {
  children: ReactNode;
  icon?: Icon;
  className?: string;
}) {
  return (
    <p
      className={cn(
        "m-0 mb-6 inline-flex items-center gap-2 rounded-pill border border-site-light-100 bg-site-light-040 py-1.5 pr-3.5 pl-3 font-mono text-[12.5px] text-fg/85 backdrop-blur-md",
        className
      )}
    >
      {Glyph ? <Glyph className="size-3.5 text-brand-strong" aria-hidden /> : null}
      {children}
    </p>
  );
}

const DISPLAY = "m-0 font-display font-normal tracking-[-0.03em] text-balance text-fg";
const H2_SIZE = {
  sm: "md:text-[52px] md:leading-[1.05]",
  md: "md:text-[64px] md:leading-[1.03]",
  lg: "md:text-[80px] md:leading-[1.0]",
  xl: "md:text-[120px] md:leading-[0.96]",
};

export function H1({ children }: { children: ReactNode }) {
  return (
    <h1 className={cn(DISPLAY, "text-[46px] leading-[1.03] md:text-[92px] md:leading-[0.98]")}>
      {children}
    </h1>
  );
}

export function H2({
  children,
  size = "md",
  className,
}: {
  children: ReactNode;
  size?: keyof typeof H2_SIZE;
  className?: string;
}) {
  return (
    <h2 className={cn(DISPLAY, "text-[38px] leading-[1.06]", H2_SIZE[size], className)}>
      {children}
    </h2>
  );
}

export function H3({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <h3 className={cn(DISPLAY, "text-[24px]/[1.15] md:text-[30px]/[1.12]", className)}>
      {children}
    </h3>
  );
}

export function Lead({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <p
      className={cn(
        "m-0 mt-6 text-[18px]/[1.5] text-pretty text-muted-foreground md:text-[21px]/[1.45]",
        className
      )}
    >
      {children}
    </p>
  );
}

export function Muted({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <p className={cn("m-0 text-[15.5px]/[1.55] text-pretty text-muted-foreground", className)}>
      {children}
    </p>
  );
}

export function Small({ children, className }: { children: ReactNode; className?: string }) {
  return <p className={cn("m-0 text-[13px]/[1.5] text-subtle", className)}>{children}</p>;
}

export function TextLink({
  children,
  href,
  className,
}: {
  children: ReactNode;
  href: string;
  className?: string;
}) {
  const content = (
    <>
      <span className="bg-[linear-gradient(currentColor,currentColor)] bg-[length:0%_1px] bg-left-bottom bg-no-repeat pb-0.5 transition-[background-size] duration-300 group-hover/link:bg-[length:100%_1px]">
        {children}
      </span>
      <span
        aria-hidden
        className="transition-transform duration-300 group-hover/link:translate-x-1"
      >
        →
      </span>
    </>
  );
  const linkClass = cn(
    "group/link inline-flex items-center gap-1.5 text-[15px] text-fg",
    className
  );
  return isExternalHref(href) ? (
    <a href={href} target="_blank" rel="noopener noreferrer" className={linkClass}>
      {content}
    </a>
  ) : (
    <Link href={href} className={linkClass}>
      {content}
    </Link>
  );
}

/** Calls to action render as links when given `href`, as buttons otherwise. */
interface CtaProps {
  children: ReactNode;
  icon?: Icon;
  href?: string;
  className?: string;
}

type CtaVariant = "primary" | "outline";

/**
 * One call to action: a real link (no button role) when it navigates, a button otherwise. The link
 * wears the button's classes, so both look the same; external URLs open in a new tab.
 */
function Cta({
  children,
  href,
  variant,
  className,
}: {
  children: ReactNode;
  href?: string;
  variant: CtaVariant;
  className: string;
}) {
  if (href === undefined) {
    return (
      <Button size="cta-lg" variant={variant} className={className}>
        {children}
      </Button>
    );
  }
  const linkClass = cn(buttonVariants({ size: "cta-lg", variant }), className);
  return isExternalHref(href) ? (
    <Link
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      data-slot="button"
      className={linkClass}
    >
      {children}
    </Link>
  ) : (
    <Link href={href} data-slot="button" className={linkClass}>
      {children}
    </Link>
  );
}

export function PrimaryCta({ children, icon: Glyph, href, className }: CtaProps) {
  return (
    <Cta
      href={href}
      variant="primary"
      className={cn(
        "group/cta relative h-[52px] overflow-hidden bg-[image:var(--gradient-brand)] bg-[length:190%_100%] bg-left px-7 text-[15.5px] text-[color:var(--color-on-gradient)] shadow-[0_0_44px_-6px_rgba(29,240,215,0.5)] transition-[background-position,box-shadow] duration-500 hover:bg-right hover:shadow-[0_0_56px_-4px_rgba(29,240,215,0.75)]",
        className
      )}
    >
      <span
        aria-hidden
        className="absolute inset-y-0 -left-1/3 w-1/3 -skew-x-12 bg-site-light-350 opacity-0 transition-[left,opacity] duration-700 group-hover/cta:left-full group-hover/cta:opacity-100"
      />
      {Glyph ? <Glyph /> : null}
      {children}
    </Cta>
  );
}

export function SecondaryCta({ children, icon: Glyph, href, className }: CtaProps) {
  return (
    <Cta
      href={href}
      variant="outline"
      className={cn(
        "h-[52px] border-site-light-150 bg-site-light-030 px-7 text-[15.5px] backdrop-blur-md",
        className
      )}
    >
      {Glyph ? <Glyph /> : null}
      {children}
    </Cta>
  );
}
