"use client";

import { mergeProps } from "@base-ui/react/merge-props";
import { useRender } from "@base-ui/react/use-render";
import type { VariantProps } from "class-variance-authority";
import { useReducedMotionConfig } from "motion/react";
import * as React from "react";

import { toneBg } from "../../lib/tone";
import { cn } from "../../lib/utils";
import { pillVariants } from "./pill-variants";
import type { PillSize, PillTone } from "./pill-types";

type PillContextValue = {
  size: PillSize;
  mono: boolean;
  tone: PillTone;
  pulse: boolean;
};

const PillContext = React.createContext<PillContextValue | null>(null);

type PillVariantOptions = VariantProps<typeof pillVariants>;

export interface PillProps
  extends
    Omit<useRender.ComponentProps<"span">, "color">,
    Pick<PillVariantOptions, "tone" | "size" | "mono" | "solid"> {
  /** Toggle state. Only meaningful when the Pill is rendered as a control (e.g. `render={<button />}`). */
  active?: boolean;
  /** Propagate a pulsing state to the inner `Pill.Dot` when the dot does not set `pulse` itself. */
  pulse?: boolean;
}

export type PillLinkProps = Omit<PillProps, "active" | "render" | "solid"> &
  Omit<React.ComponentProps<"a">, keyof PillProps | "color"> & {
    render?: PillProps["render"];
    solid?: PillProps["solid"];
  };

function Pill({
  tone: toneProp,
  size: sizeProp,
  mono: monoProp,
  solid: solidProp,
  active,
  pulse,
  className,
  render,
  ...props
}: PillProps) {
  const tone: PillTone = toneProp ?? "neutral";
  const size: PillSize = sizeProp ?? "sm";
  const mono = Boolean(monoProp);
  const solid = Boolean(solidProp);
  const ctx: PillContextValue = { size, mono, tone, pulse: Boolean(pulse) };
  const element = useRender({
    defaultTagName: "span",
    props: mergeProps<"span">(
      {
        className: cn(pillVariants({ tone, size, mono, solid, active }), className),
      } as Record<string, unknown>,
      {
        "data-slot": "pill",
        "data-tone": tone,
        "data-size": size,
        "data-mono": mono ? "true" : undefined,
        "data-solid": solid ? "true" : undefined,
        "data-active": active === true ? "true" : active === false ? "false" : undefined,
        "data-pulse": pulse ? "true" : undefined,
        "aria-pressed": active === true || active === false ? active : undefined,
      } as Record<string, unknown>,
      props
    ),
    render,
    state: { slot: "pill", tone, size, mono, solid, active },
  });
  return <PillContext.Provider value={ctx}>{element}</PillContext.Provider>;
}

export interface PillDotProps extends Omit<React.ComponentProps<"span">, "color"> {
  tone?: PillTone;
  /** CSS color or `var(...)` reference. Overrides `tone`-derived color. */
  color?: string;
  pulse?: boolean;
  size?: "sm" | "md";
}

function PillDot({
  tone,
  color,
  pulse,
  size: explicitSize,
  className,
  style,
  ...props
}: PillDotProps) {
  const ctx = React.use(PillContext);
  const reduced = useReducedMotionConfig();
  const effectivePulse = pulse ?? ctx?.pulse ?? false;
  const shouldAnimate = effectivePulse && !reduced;
  const effectiveSize: "sm" | "md" =
    explicitSize ?? (ctx ? (ctx.size === "md" ? "md" : "sm") : "md");
  const effectiveTone: PillTone = tone ?? ctx?.tone ?? "neutral";
  return (
    <span
      aria-hidden="true"
      data-slot="pill-dot"
      data-tone={effectiveTone}
      data-size={effectiveSize}
      data-pulse={shouldAnimate ? "true" : undefined}
      className={cn(
        "inline-block shrink-0 rounded-full",
        effectiveSize === "sm" ? "size-1.5" : "size-2",
        color === undefined && toneBg(effectiveTone),
        shouldAnimate && "animate-pulse",
        className
      )}
      style={color === undefined ? style : { backgroundColor: color, ...style }}
      {...props}
    />
  );
}

function PillLink({
  tone = "accent",
  size = "sm",
  mono = true,
  className,
  href,
  render,
  children,
  ...props
}: PillLinkProps) {
  return (
    <Pill
      tone={tone}
      size={size}
      mono={mono}
      className={cn("hover:text-accent-strong", className)}
      render={
        render ?? (
          <a href={href ?? "#"} aria-label={typeof children === "string" ? children : undefined} />
        )
      }
      {...props}
    >
      {children}
    </Pill>
  );
}

const PillRoot = Pill as typeof Pill & { Dot: typeof PillDot; Link: typeof PillLink };
PillRoot.Dot = PillDot;
PillRoot.Link = PillLink;

export { PillRoot as Pill, PillDot, PillLink };
export type { PillSize, PillTone };
