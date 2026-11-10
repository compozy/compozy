"use client";

/** The interactive and motion-backed marketing primitives: install command, product camera, windows, plates. */
import { Button, cn } from "@compozy/ui";
import { Check, Copy } from "@compozy/ui/icons";
import { m, useReducedMotion, type MotionStyle } from "motion/react";
import { useState, type ReactNode } from "react";

import { WHOLE, focusToTransform, type Focus } from "@/components/site/layout";

export function InstallCommand({ command, className }: { command: string; className?: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div
      className={cn(
        "inline-flex h-[52px] max-w-full items-center gap-3 rounded-pill border border-site-light-120 bg-site-dark-400 pr-2 pl-5 backdrop-blur-md",
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
          "bg-site-light-050 shadow-[0_0_0_1px_rgba(255,255,255,0.09),0_50px_140px_-30px_rgba(0,0,0,0.95),0_0_120px_-30px_rgba(31,135,252,0.45)] backdrop-blur-xl",
        className
      )}
    >
      <div
        className={cn(
          "relative overflow-hidden rounded-[16px] border border-site-light-100 bg-[#131313]",
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
