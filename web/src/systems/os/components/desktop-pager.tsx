import { Ellipsis } from "lucide-react";
import { useRef, useState } from "react";
import type * as React from "react";

import { Button, Tooltip, TooltipContent, TooltipProvider, TooltipTrigger, cn } from "@agh/ui";

export interface DesktopPagerItem {
  id: string;
  name: string;
}

export interface DesktopPagerOverflowRequest {
  direction: "earlier" | "later";
  hiddenDesktopIds: readonly string[];
  anchorDesktopId: string;
}

export interface DesktopPagerProps extends Omit<React.ComponentProps<"nav">, "children"> {
  desktops: readonly DesktopPagerItem[];
  activeDesktopId: string;
  /** Compact presentation changes containment only; navigation cardinality stays identical. */
  compact?: boolean;
  /** Desktop switches stay unavailable until the daemon has a live registered client. */
  canSwitchDesktop?: boolean;
  onSelectDesktop: (desktopId: string) => void;
  onOpenOverview: (request: DesktopPagerOverflowRequest) => void;
}

interface DesktopControl {
  kind: "desktop";
  key: string;
  desktop: DesktopPagerItem;
  position: number;
}

interface OverflowControl {
  kind: "overflow";
  key: string;
  direction: "earlier" | "later";
  hiddenDesktopIds: readonly string[];
}

type PagerControl = DesktopControl | OverflowControl;

const ALL_DESKTOPS_LIMIT = 7;
const WINDOW_RADIUS = 2;
const WINDOW_SIZE = WINDOW_RADIUS * 2 + 1;

function pagerControls(
  desktops: readonly DesktopPagerItem[],
  activeDesktopId: string
): PagerControl[] {
  const activePosition = Math.max(
    0,
    desktops.findIndex(desktop => desktop.id === activeDesktopId)
  );

  if (desktops.length <= ALL_DESKTOPS_LIMIT) {
    return desktops.map((desktop, position) => ({
      kind: "desktop",
      key: `desktop:${desktop.id}`,
      desktop,
      position,
    }));
  }

  const start = Math.min(
    Math.max(activePosition - WINDOW_RADIUS, 0),
    desktops.length - WINDOW_SIZE
  );
  const end = start + WINDOW_SIZE;
  const controls: PagerControl[] = [];
  const earlier = desktops.slice(0, start);
  const later = desktops.slice(end);

  if (earlier.length > 0) {
    controls.push({
      kind: "overflow",
      key: "overflow:earlier",
      direction: "earlier",
      hiddenDesktopIds: earlier.map(desktop => desktop.id),
    });
  }

  desktops.slice(start, end).forEach((desktop, offset) => {
    controls.push({
      kind: "desktop",
      key: `desktop:${desktop.id}`,
      desktop,
      position: start + offset,
    });
  });

  if (later.length > 0) {
    controls.push({
      kind: "overflow",
      key: "overflow:later",
      direction: "later",
      hiddenDesktopIds: later.map(desktop => desktop.id),
    });
  }

  return controls;
}

function overflowLabel(control: OverflowControl): string {
  const count = control.hiddenDesktopIds.length;
  return `Show ${count} ${control.direction} desktop${count === 1 ? "" : "s"}`;
}

/**
 * Bottom-chrome desktop navigation. The 44px controls remain transparent at rest;
 * only the 6px dots and 14px active pill are visible.
 */
export function DesktopPager({
  desktops,
  activeDesktopId,
  compact = false,
  canSwitchDesktop = true,
  onSelectDesktop,
  onOpenOverview,
  className,
  "aria-label": ariaLabel = "Desktops",
  ...props
}: DesktopPagerProps) {
  const controls = pagerControls(desktops, activeDesktopId);
  const controlRefs = useRef(new Map<string, HTMLButtonElement>());
  const [rovingKey, setRovingKey] = useState<string>();
  const activeKey = `desktop:${activeDesktopId}`;
  const visibleKeys = new Set(controls.map(control => control.key));
  const tabStopKey = rovingKey && visibleKeys.has(rovingKey) ? rovingKey : activeKey;

  if (desktops.length === 0) return null;

  const focusControl = (position: number) => {
    const control = controls[position];
    if (!control) return;
    setRovingKey(control.key);
    controlRefs.current.get(control.key)?.focus();
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>, key: string) => {
    const position = controls.findIndex(control => control.key === key);
    if (position < 0) return;

    let nextPosition: number | null = null;
    if (event.key === "ArrowLeft") nextPosition = Math.max(0, position - 1);
    if (event.key === "ArrowRight") nextPosition = Math.min(controls.length - 1, position + 1);
    if (event.key === "Home") nextPosition = 0;
    if (event.key === "End") nextPosition = controls.length - 1;
    if (nextPosition === null) return;

    event.preventDefault();
    focusControl(nextPosition);
  };

  return (
    <nav
      aria-label={ariaLabel}
      data-slot="desktop-pager"
      data-presentation={compact ? "compact" : "floating"}
      className={cn(
        "no-scrollbar min-w-0 overflow-x-auto overscroll-x-contain p-0.5",
        compact ? "w-[50vw] max-w-[50vw]" : "w-full max-w-full",
        className
      )}
      {...props}
    >
      <TooltipProvider>
        <ol className="flex w-max items-center" aria-label="Desktop positions">
          {controls.map(control => {
            const label =
              control.kind === "desktop"
                ? `Desktop ${control.position + 1} of ${desktops.length}: ${control.desktop.name}`
                : overflowLabel(control);
            const active = control.kind === "desktop" && control.desktop.id === activeDesktopId;

            return (
              <li key={control.key}>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        ref={node => {
                          if (node) controlRefs.current.set(control.key, node);
                          else controlRefs.current.delete(control.key);
                        }}
                        type="button"
                        variant="ghost"
                        aria-current={active ? "page" : undefined}
                        aria-disabled={
                          control.kind === "desktop" && !canSwitchDesktop ? true : undefined
                        }
                        aria-label={label}
                        data-active={active ? "true" : undefined}
                        data-direction={control.kind === "overflow" ? control.direction : undefined}
                        data-slot={
                          control.kind === "desktop"
                            ? "desktop-pager-control"
                            : "desktop-pager-overflow"
                        }
                        tabIndex={control.key === tabStopKey ? 0 : -1}
                        className={cn(
                          "relative size-11 rounded-pill p-0 hover:bg-transparent",
                          "focus-visible:bg-transparent focus-visible:shadow-focus-ring",
                          "aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
                        )}
                        onClick={() => {
                          setRovingKey(control.key);
                          if (control.kind === "desktop") {
                            if (!canSwitchDesktop) return;
                            if (!active) onSelectDesktop(control.desktop.id);
                            return;
                          }
                          onOpenOverview({
                            direction: control.direction,
                            hiddenDesktopIds: control.hiddenDesktopIds,
                            anchorDesktopId: activeDesktopId,
                          });
                        }}
                        onFocus={() => setRovingKey(control.key)}
                        onKeyDown={event => handleKeyDown(event, control.key)}
                      />
                    }
                  >
                    {control.kind === "desktop" ? (
                      <span aria-hidden="true" className="relative block size-3.5">
                        <span
                          className={cn(
                            "absolute top-1/2 left-1/2 size-1.5 -translate-x-1/2 -translate-y-1/2 rounded-pill bg-neutral-ink",
                            "transition-opacity duration-shell-fast ease-out group-hover/button:bg-muted",
                            active ? "opacity-0" : "opacity-100"
                          )}
                        />
                        <span
                          className={cn(
                            "absolute top-1/2 left-1/2 h-1.5 w-3.5 -translate-x-1/2 -translate-y-1/2 rounded-pill bg-fg-strong",
                            "transition-opacity duration-shell-fast ease-out",
                            active ? "opacity-100" : "opacity-0"
                          )}
                        />
                      </span>
                    ) : (
                      <Ellipsis aria-hidden="true" className="size-3.5 text-neutral-ink" />
                    )}
                  </TooltipTrigger>
                  <TooltipContent side="top">{label}</TooltipContent>
                </Tooltip>
              </li>
            );
          })}
        </ol>
      </TooltipProvider>
    </nav>
  );
}
