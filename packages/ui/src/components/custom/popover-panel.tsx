"use client";

import type * as React from "react";

import { X } from "../../icons";
import { cn } from "../../lib/utils";
import { Button } from "../button";
import { PopoverContent, PopoverDescription, PopoverTitle } from "../popover";
import { KindIcon, type KindIconProps } from "./kind-icon";

export type PopoverPanelTone = "neutral" | "warning" | "destructive" | "success";

const WELL_TONE: Record<PopoverPanelTone, string | undefined> = {
  neutral: undefined,
  warning: "bg-warning-tint text-warning",
  destructive: "bg-destructive-tint text-destructive",
  success: "bg-success-tint text-success",
};

type PopoverPanelContentProps = React.ComponentProps<typeof PopoverContent>;

/**
 * The popup a `PopoverPanel` renders in: the fixed panel width with no padding of its own, so the
 * panel's ruled header, body, aside and footer run edge to edge.
 */
function PopoverPanelContent({ className, ...props }: PopoverPanelContentProps) {
  return (
    <PopoverContent
      data-slot="popover-panel-content"
      className={cn("w-80 max-w-[calc(100vw-2rem)] gap-0 p-0", className)}
      {...props}
    />
  );
}

type PopoverPanelGlyph = NonNullable<KindIconProps["icon"]>;

/**
 * The header identity: a glyph in a toned well, or a mark that replaces the well (e.g. a brand
 * logo plate composed through `KindIcon`).
 */
type PopoverPanelIdentity =
  | {
      /** Glyph in the header well. */
      icon: PopoverPanelGlyph;
      mark?: undefined;
    }
  | {
      icon?: undefined;
      /** A mark that replaces the glyph well. */
      mark: React.ReactNode;
    };

interface PopoverPanelBaseProps extends Omit<React.ComponentProps<"div">, "title"> {
  /** Tints the well: the panel's state at a glance. */
  tone?: PopoverPanelTone;
  /** The popup's accessible name. */
  title: React.ReactNode;
  /** One quiet line under the title; it describes the popup. */
  subtitle?: React.ReactNode;
  /** Renders the header close button. */
  onClose?: () => void;
  closeLabel?: string;
  /** Extra ruled section under the body (a folded fallback, a secondary fact). */
  aside?: React.ReactNode;
  /** Ruled action row. */
  footer?: React.ReactNode;
}

export type PopoverPanelProps = PopoverPanelBaseProps & PopoverPanelIdentity;

/**
 * A decision panel inside a popover: an identity well, a title with one quiet subtitle, a hairline,
 * the body, an optional ruled aside, and a ruled footer with the actions. Render it inside
 * `PopoverPanelContent`; the title names the popup.
 */
function PopoverPanel({
  icon,
  mark,
  tone = "neutral",
  title,
  subtitle,
  onClose,
  closeLabel = "Close",
  aside,
  footer,
  className,
  children,
  ...props
}: PopoverPanelProps) {
  return (
    <div data-slot="popover-panel" className={cn("flex flex-col", className)} {...props}>
      <header
        data-slot="popover-panel-header"
        className="flex items-start gap-3 border-b border-line-soft px-3.5 py-3"
      >
        {mark ?? (
          <KindIcon
            aria-hidden="true"
            icon={icon}
            tone="well"
            size="sm"
            className={cn("shrink-0", WELL_TONE[tone])}
            data-tone={tone}
          />
        )}
        <div className="flex min-w-0 flex-1 flex-col gap-0.5 pt-px">
          <PopoverTitle className="truncate text-small-body font-semibold text-fg-strong">
            {title}
          </PopoverTitle>
          {subtitle ? (
            <PopoverDescription className="truncate text-meta text-faint">
              {subtitle}
            </PopoverDescription>
          ) : null}
        </div>
        {onClose ? (
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            aria-label={closeLabel}
            className="-mt-0.5 -mr-1 text-faint"
            onClick={onClose}
          >
            <X />
          </Button>
        ) : null}
      </header>
      <div data-slot="popover-panel-body" className="flex flex-col gap-3 px-3.5 py-3.5">
        {children}
      </div>
      {aside ? (
        <div data-slot="popover-panel-aside" className="border-t border-line-soft px-3.5 py-2.5">
          {aside}
        </div>
      ) : null}
      {footer ? (
        <footer
          data-slot="popover-panel-footer"
          className="flex items-center gap-1.5 border-t border-line-soft px-3.5 py-2.5"
        >
          {footer}
        </footer>
      ) : null}
    </div>
  );
}

export { PopoverPanel, PopoverPanelContent };
export type { PopoverPanelContentProps };
