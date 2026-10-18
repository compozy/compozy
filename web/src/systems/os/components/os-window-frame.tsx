import { Topbar, TopbarSlotProvider, useTopbarSlotValue, type TopbarSlotStore } from "@compozy/ui";
import * as React from "react";

import { cn } from "@/lib/utils";

import { OsTrafficLights, type OsTrafficLightAction } from "./os-traffic-lights";

/**
 * Outer shell chrome for one window frame (solo window or tab group). Tiled
 * frames are flat surface boxes: the seams between them are the only divider.
 * Floating frames lift off the desk with a `line` hairline and the elevated
 * shadow. Focus is carried by the head (blurred windows dim their identity and
 * trail), never by the frame edge. Presentational only — drag, z-order, and
 * focus come from the window manager.
 */
export interface OsWindowChromeProps extends React.ComponentProps<"section"> {
  focused?: boolean;
  /** Compact (<960px): full-bleed stack surface — no border/shadow. */
  presentation?: "floating" | "compact";
  /** Tiled panes are flush with their seams; floating frames lift off the desk. */
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
  const floating = presentation !== "compact" && kind === "floating";
  return (
    <section
      data-slot="os-window-frame"
      data-focused={focused ? "" : undefined}
      data-presentation={presentation === "compact" ? "compact" : undefined}
      data-kind={kind}
      className={cn(
        "flex min-h-0 flex-col overflow-hidden bg-canvas",
        floating && "border border-line shadow-elevated",
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
      className="no-scrollbar flex h-window-toolbar shrink-0 items-center gap-2.5 overflow-x-auto border-b border-line bg-canvas px-3 [&_[data-slot=listing-toolbar]]:w-full [&_[data-slot=listing-toolbar]]:flex-nowrap"
    >
      {slot.toolbar}
    </div>
  );
}

function OsWindowBody({
  children,
  onScrolled,
  transitionName,
}: {
  children: React.ReactNode;
  onScrolled: (scrolled: boolean) => void;
  transitionName?: string;
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
      style={transitionName ? { viewTransitionName: transitionName } : undefined}
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
  /** Deck frames omit head controls — the deck row owns the window controls. */
  controls?: "head" | "deck";
  onTrafficLight?: (action: OsTrafficLightAction) => void;
  zoomMenu?: (button: React.ReactNode) => React.ReactNode;
  /** The owning frame fills its desktop; the head's zoom control reads as pressed. */
  zoomed?: boolean;
  headClassName?: string;
  presentation?: "floating" | "compact";
  slotStore?: TopbarSlotStore;
  /**
   * `view-transition-name` for the body. Only the focused, visible window may
   * carry one — names must be unique per document or the transition aborts.
   */
  bodyTransitionName?: string;
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
  bodyTransitionName,
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
          controls={
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
            scrolled && "border-line-strong",
            !focused &&
              "[&_[data-slot=topbar-identity]]:opacity-55 [&_[data-slot=topbar-trailing]]:opacity-55",
            headClassName
          )}
        />
        <OsWindowToolbar />
        <OsWindowBody onScrolled={setScrolled} transitionName={bodyTransitionName}>
          {children}
        </OsWindowBody>
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
