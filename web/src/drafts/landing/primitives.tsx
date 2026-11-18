/** Landing type ramp, framing and the product camera: marketing scale on the shipped tokens. */
import { Button, cn } from "@compozy/ui";
import { Check, Copy } from "@compozy/ui/icons";
import { m, useReducedMotion, type MotionStyle } from "motion/react";
import { useState, type ComponentType, type ReactNode, type SVGProps } from "react";

import { WHOLE, focusToTransform, type Focus } from "./layout";

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
        "m-0 mb-6 inline-flex items-center gap-2 rounded-pill border border-[color:var(--light-100)] bg-[color:var(--light-040)] py-1.5 pr-3.5 pl-3 font-mono text-[12.5px] text-fg/85 backdrop-blur-md",
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

export function TextLink({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <a
      href="#"
      className={cn("group/link inline-flex items-center gap-1.5 text-[15px] text-fg", className)}
    >
      <span className="bg-[linear-gradient(currentColor,currentColor)] bg-[length:0%_1px] bg-left-bottom bg-no-repeat pb-0.5 transition-[background-size] duration-300 group-hover/link:bg-[length:100%_1px]">
        {children}
      </span>
      <span
        aria-hidden
        className="transition-transform duration-300 group-hover/link:translate-x-1"
      >
        →
      </span>
    </a>
  );
}

export function PrimaryCta({ children, icon: Glyph }: { children: ReactNode; icon?: Icon }) {
  return (
    <Button
      size="cta-lg"
      variant="primary"
      className="group/cta relative h-[52px] overflow-hidden bg-[image:var(--gradient-brand)] bg-[length:190%_100%] bg-left px-7 text-[15.5px] text-[color:var(--color-on-gradient)] shadow-[0_0_44px_-6px_rgba(29,240,215,0.5)] transition-[background-position,box-shadow] duration-500 hover:bg-right hover:shadow-[0_0_56px_-4px_rgba(29,240,215,0.75)]"
    >
      <span
        aria-hidden
        className="absolute inset-y-0 -left-1/3 w-1/3 -skew-x-12 bg-[color:var(--light-350)] opacity-0 transition-[left,opacity] duration-700 group-hover/cta:left-full group-hover/cta:opacity-100"
      />
      {Glyph ? <Glyph /> : null}
      {children}
    </Button>
  );
}

export function SecondaryCta({ children, icon: Glyph }: { children: ReactNode; icon?: Icon }) {
  return (
    <Button
      size="cta-lg"
      variant="outline"
      className="h-[52px] border-[color:var(--light-150)] bg-[color:var(--light-030)] px-7 text-[15.5px] backdrop-blur-md"
    >
      {Glyph ? <Glyph /> : null}
      {children}
    </Button>
  );
}

export function InstallCommand({ command, className }: { command: string; className?: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div
      className={cn(
        "inline-flex h-[52px] max-w-full items-center gap-3 rounded-pill border border-[color:var(--light-120)] bg-[color:var(--dark-400)] pr-2 pl-5 backdrop-blur-md",
        className
      )}
    >
      <span aria-hidden className="font-mono text-[13.5px] text-brand-strong">
        $
      </span>
      <code className="truncate font-mono text-[13.5px] text-fg">{command}</code>
      <Button
        aria-label={copied ? "Copied" : "Copy command"}
        size="icon-sm"
        variant="quiet"
        onClick={() => {
          void navigator.clipboard?.writeText(command);
          setCopied(true);
        }}
      >
        {copied ? <Check className="text-brand-strong" /> : <Copy />}
      </Button>
    </div>
  );
}

/** A real capture filmed through a camera: it moves to the part of the screen that matters. */
export function Camera({
  src,
  alt,
  focus = WHOLE,
  active = true,
  className,
  duration = 1.5,
}: {
  src: string;
  alt: string;
  focus?: Focus;
  active?: boolean;
  className?: string;
  duration?: number;
}) {
  const reduced = useReducedMotion();
  return (
    <m.div
      className={cn("absolute inset-0", className)}
      initial={false}
      animate={{ opacity: active ? 1 : 0 }}
      transition={{ duration: reduced ? 0 : 0.7 }}
      aria-hidden={!active}
    >
      <m.img
        src={src}
        alt={active ? alt : ""}
        draggable={false}
        className="block size-full origin-top-left object-cover"
        initial={false}
        animate={focusToTransform(focus)}
        transition={{ duration: reduced ? 0 : duration, ease: [0.16, 1, 0.3, 1] }}
      />
    </m.div>
  );
}

/** The window every capture sits in: real depth, a lit edge, room to be read. */
export function ProductWindow({
  children,
  className,
  style,
  glow = true,
  aspect = "aspect-[16/10]",
}: {
  children: ReactNode;
  className?: string;
  style?: MotionStyle;
  glow?: boolean;
  aspect?: string;
}) {
  return (
    <m.div
      style={style}
      className={cn(
        "relative rounded-[20px] p-[5px]",
        glow &&
          "bg-[color:var(--light-050)] shadow-[0_0_0_1px_rgba(255,255,255,0.09),0_50px_140px_-30px_rgba(0,0,0,0.95),0_0_120px_-30px_rgba(31,135,252,0.45)] backdrop-blur-xl",
        className
      )}
    >
      <div
        className={cn(
          "relative overflow-hidden rounded-[16px] border border-[color:var(--light-100)] bg-[#131313]",
          aspect
        )}
      >
        {children}
      </div>
    </m.div>
  );
}

export function Shot({
  src,
  alt,
  className,
  aspect,
}: {
  src: string;
  alt: string;
  className?: string;
  aspect?: string;
}) {
  return (
    <ProductWindow className={className} aspect={aspect}>
      <img
        src={src}
        alt={alt}
        draggable={false}
        className="block size-full object-cover object-top"
      />
    </ProductWindow>
  );
}

/** Generated brand art behind a section. Screen blending lets its black fall away into the page. */
export function Plate({
  src,
  className,
  style,
}: {
  src: string;
  className?: string;
  style?: MotionStyle;
}) {
  return (
    <m.img
      src={src}
      alt=""
      aria-hidden
      draggable={false}
      style={style}
      className={cn(
        "pointer-events-none absolute select-none object-cover mix-blend-screen",
        className
      )}
    />
  );
}
