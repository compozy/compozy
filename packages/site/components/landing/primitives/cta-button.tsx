import type { ReactNode } from "react";
import Link from "next/link";
import { buttonVariants } from "@agh/ui";
import { cn } from "@agh/ui/lib/utils";

type Variant = "primary" | "ghost";
type ButtonVariantsProps = Parameters<typeof buttonVariants>[0];

const VARIANT_PROPS: Record<Variant, ButtonVariantsProps> = {
  // Primary uses the design system's default (accent bg + foreground text).
  primary: { variant: "default", size: "lg" },
  // Ghost maps to outline , more visible than ui's hover-only ghost on dark bg.
  ghost: { variant: "outline", size: "lg" },
};

// Landing CTAs need extra horizontal weight than the app's dense defaults.
const HERO_SIZE_OVERRIDE = "px-5";

// Keep the hover language consistent with the landing's accent-forward system.
const GHOST_HOVER_OVERRIDE =
  "hover:border-accent hover:bg-transparent hover:text-accent dark:hover:bg-transparent";

interface CtaButtonProps {
  href: string;
  children: ReactNode;
  variant?: Variant;
  className?: string;
  external?: boolean;
}

export function CtaButton({
  href,
  children,
  variant = "primary",
  className,
  external = false,
}: CtaButtonProps) {
  const classes = cn(
    buttonVariants(VARIANT_PROPS[variant]),
    HERO_SIZE_OVERRIDE,
    variant === "ghost" && GHOST_HOVER_OVERRIDE,
    className
  );

  if (external) {
    return (
      <a href={href} target="_blank" rel="noopener noreferrer" className={classes}>
        {children}
      </a>
    );
  }

  return (
    <Link href={href} className={classes}>
      {children}
    </Link>
  );
}
