import { Topbar, TopbarSlotProvider, useTopbarSlotValue, type TopbarSlotStore } from "@compozy/ui";
import * as React from "react";

import { cn } from "@/lib/utils";

import { OsTrafficLights, type OsTrafficLightAction } from "./os-traffic-lights";

/**
 * Outer shell chrome for one window frame (solo window or tab group): border,
 * radius, and (for floating frames) the cast window shadow are the sanctioned
 * shell carve-out. Tiled frames keep radius and border without a cast that
 * cannot fit the layout gutters. Everything inside stays flat. Presentational
 * only — drag, z-order, and focus come from the window manager.
 */
export interface OsWindowChromeProps extends React.ComponentProps<"section"> {
  focused?: boolean;
  /** Compact (<960px): full-bleed stack surface — no border/radius/shadow. */
  presentation?: "floating" | "compact";
  /** Tiled panes sit in the work-area gutters; floating frames lift off the wallpaper. */
  kind?: "tiled" | "floating";
}

function targetsWindowChromeControl(target: EventTarget | null): boolean {
  return (
    target instanceof Element &&
    target.closest('[data-slot="os-traffic-lights"] button[data-action]') !== null
  );
}

export function OsWindowChrome({
  focused = true,
  presentation = "floating",
  kind = "floating",
  className,
  children,
  onPointerDownCapture,
  ...props
}: OsWindowChromeProps) {
  const compact = presentation === "compact";
  const tiled = kind === "tiled";
  return (
    <section
      data-slot="os-window-frame"
      data-focused={focused ? "" : undefined}
      data-presentation={compact ? "compact" : undefined}
      data-kind={kind}
      className={cn(
        "flex min-h-0 flex-col overflow-hidden bg-canvas",
        compact
          ? "rounded-none border-0 shadow-none"
          : [
              "rounded-window border",
              focused ? "border-line-focus" : "border-line-strong",
              tiled ? "shadow-none" : focused ? "shadow-window" : "shadow-window-unfocused",
            ],
        className
      )}
      onPointerDownCapture={event => {
        if (targetsWindowChromeControl(event.target)) return;
        onPointerDownCapture?.(event);
      }}
      {...props}
    >
      {children}
    </section>
  );
}

function OsWindowToolbar() {
  const slot = useTopbarSlotValue();
  if (!slot?.toolbar) return null;
  return (
    <div
      data-slot="os-window-toolbar"
      className="no-scrollbar flex h-[38px] shrink-0 items-center gap-2.5 overflow-x-auto border-b border-line bg-canvas px-3 py-0.5 [&_[data-slot=listing-toolbar]]:w-full [&_[data-slot=listing-toolbar]]:flex-nowrap"
    >
      {slot.toolbar}
    </div>
  );
}

function OsWindowBody({
  children,
  onScrolled,
}: {
  children: React.ReactNode;
  onScrolled: (scrolled: boolean) => void;
}) {
  const handleScroll = (event: React.UIEvent<HTMLDivElement>) => {
    const scrollTarget = event.target;
    if (!(scrollTarget instanceof HTMLElement)) return;
    onScrolled(scrollTarget.scrollTop > 2);
  };

  return (
    <div
      data-slot="os-window-body"
      className="flex min-h-0 flex-1 flex-col overflow-auto bg-canvas"
      onScrollCapture={handleScroll}
    >
      {children}
    </div>
  );
}

/**
 * One member's head + strip + body (D2: the head belongs entirely to the
 * active tab). The owning frame supplies the member's topbar slot store so
 * hidden members keep publishing and the deck can read live leaf labels.
 */
export interface OsWindowSurfaceProps extends Omit<React.ComponentProps<"section">, "title"> {
  title: React.ReactNode;
  glyph?: React.ReactNode;
  focused?: boolean;
  /** Deck frames omit head controls — the deck row owns the traffic lights. */
  controls?: "head" | "deck";
  onTrafficLight?: (action: OsTrafficLightAction) => void;
  zoomMenu?: (button: React.ReactNode) => React.ReactNode;
  /** The owning frame fills its desktop; the head's zoom control reads as pressed. */
  zoomed?: boolean;
  headClassName?: string;
  presentation?: "floating" | "compact";
  slotStore?: TopbarSlotStore;
}

export function OsWindowSurface({
  title,
  glyph,
  focused = true,
  controls = "head",
  onTrafficLight,
  zoomMenu,
  zoomed = false,
  headClassName,
  presentation = "floating",
  slotStore,
  className,
  children,
  ...props
}: OsWindowSurfaceProps) {
  const [scrolled, setScrolled] = React.useState(false);
  const compact = presentation === "compact";

  return (
    <section
      data-slot="os-window-surface"
      className={cn("flex min-h-0 flex-1 flex-col overflow-hidden bg-canvas", className)}
      {...props}
    >
      <TopbarSlotProvider store={slotStore}>
        <Topbar
          data-slot="os-window-head"
          data-scrolled={scrolled ? "" : undefined}
          leading={
            controls === "head" ? (
              <OsTrafficLights
                onSelect={onTrafficLight}
                compact={compact}
                wrapZoom={zoomMenu}
                zoomed={zoomed}
              />
            ) : undefined
          }
          title={title}
          glyph={glyph}
          className={cn(
            scrolled && "border-line-strong shadow-window-head-scrolled",
            !focused &&
              "[&_[data-slot=topbar-title]]:text-subtle [&_[data-slot=topbar-identity]]:opacity-60 [&_[data-slot=topbar-trailing]]:opacity-60",
            headClassName
          )}
        />
        <OsWindowToolbar />
        <OsWindowBody onScrolled={setScrolled}>{children}</OsWindowBody>
      </TopbarSlotProvider>
    </section>
  );
}

export type OsWindowFrameProps = Omit<OsWindowChromeProps, "title"> &
  Pick<
    OsWindowSurfaceProps,
    "title" | "glyph" | "onTrafficLight" | "zoomMenu" | "zoomed" | "headClassName"
  >;

/**
 * Solo composition — today's single-window chrome, unchanged (rule D1).
 * Container props (activation captures included) land on the chrome so the
 * traffic-light guard keeps chrome clicks out of window activation.
 */
export function OsWindowFrame({
  title,
  glyph,
  focused = true,
  onTrafficLight,
  zoomMenu,
  zoomed = false,
  headClassName,
  presentation = "floating",
  children,
  ...chromeProps
}: OsWindowFrameProps) {
  return (
    <OsWindowChrome focused={focused} presentation={presentation} {...chromeProps}>
      <OsWindowSurface
        title={title}
        glyph={glyph}
        focused={focused}
        presentation={presentation}
        onTrafficLight={onTrafficLight}
        zoomMenu={zoomMenu}
        zoomed={zoomed}
        headClassName={headClassName}
      >
        {children}
      </OsWindowSurface>
    </OsWindowChrome>
  );
}
