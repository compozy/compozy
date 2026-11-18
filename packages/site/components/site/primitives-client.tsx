"use client";

/** The interactive and motion-backed marketing primitives: install command, product camera, windows, plates. */
import { CopyIconButton, cn } from "@compozy/ui";
import { m, type MotionStyle } from "motion/react";
import Image from "next/image";
import type { ReactNode } from "react";

import { useReducedMotionSafe } from "@/components/site/hooks/use-reduced-motion-safe";
import { SHOT_SIZES, WHOLE, focusToTransform, type Focus } from "@/components/site/layout";

export function InstallCommand({ command, className }: { command: string; className?: string }) {
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
      <CopyIconButton
        value={command}
        copyLabel="Copy command"
        size="icon-sm"
        variant="quiet"
        className="[&_svg]:size-3.5 data-[copied=true]:text-brand-strong data-[copy-state=failed]:text-destructive"
      />
    </div>
  );
}

/**
 * How a capture loads: `lazy` (the default) waits until it nears the viewport, `eager` loads with
 * the page, `priority` also asks for it first. Only the first viewport's art should not be lazy.
 */
export type ImageLoad = "lazy" | "eager" | "priority";

function loadProps(load: ImageLoad) {
  return load === "lazy"
    ? { loading: "lazy" as const }
    : {
        loading: "eager" as const,
        fetchPriority: load === "priority" ? ("high" as const) : undefined,
      };
}

/** A real capture filmed through a camera: it moves to the part of the screen that matters. */
export function Camera({
  src,
  alt,
  focus = WHOLE,
  active = true,
  className,
  duration = 1.5,
  sizes = SHOT_SIZES,
  load = "lazy",
}: {
  src: string;
  alt: string;
  focus?: Focus;
  active?: boolean;
  className?: string;
  duration?: number;
  sizes?: string;
  load?: ImageLoad;
}) {
  const reduced = useReducedMotionSafe();
  return (
    <m.div
      className={cn("absolute inset-0", className)}
      initial={false}
      animate={{ opacity: active ? 1 : 0 }}
      transition={{ duration: reduced ? 0 : 0.7 }}
      aria-hidden={!active}
    >
      <m.div
        className="absolute inset-0 origin-top-left"
        initial={false}
        animate={focusToTransform(focus)}
        transition={{ duration: reduced ? 0 : duration, ease: [0.16, 1, 0.3, 1] }}
      >
        <Image
          src={src}
          alt={active ? alt : ""}
          fill
          sizes={sizes}
          draggable={false}
          className="object-cover"
          {...loadProps(load)}
        />
      </m.div>
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
  sizes = SHOT_SIZES,
}: {
  src: string;
  alt: string;
  className?: string;
  aspect?: string;
  sizes?: string;
}) {
  return (
    <ProductWindow className={className} aspect={aspect}>
      <Image
        src={src}
        alt={alt}
        fill
        sizes={sizes}
        draggable={false}
        className="object-cover object-top"
      />
    </ProductWindow>
  );
}

/**
 * Generated brand art behind a section. Screen blending lets its black fall away into the page.
 * `className` places and sizes the plate; the art covers that box, framed by `imageClassName`
 * (an `object-position`, for example).
 */
export function Plate({
  src,
  className,
  imageClassName,
  style,
  sizes = "100vw",
  load = "lazy",
}: {
  src: string;
  className?: string;
  imageClassName?: string;
  style?: MotionStyle;
  sizes?: string;
  load?: ImageLoad;
}) {
  return (
    <m.div
      aria-hidden
      style={style}
      className={cn("pointer-events-none absolute select-none mix-blend-screen", className)}
    >
      <Image
        src={src}
        alt=""
        fill
        sizes={sizes}
        draggable={false}
        className={cn("object-cover", imageClassName)}
        {...loadProps(load)}
      />
    </m.div>
  );
}
