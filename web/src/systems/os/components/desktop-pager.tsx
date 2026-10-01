import { Ellipsis } from "lucide-react";
import { useState } from "react";
import type * as React from "react";

import { Button, Tooltip, TooltipContent, TooltipProvider, TooltipTrigger, cn } from "@compozy/ui";

export interface DesktopPagerItem {
  id: string;
  name: string;
  /** A window on this desktop needs you; shown only while it is off-screen. */
  needsYou?: boolean;
}

export interface DesktopPagerOverflowRequest {
  direction: "earlier" | "later";
  hiddenDesktopIds: readonly string[];
  anchorDesktopId: string;
}

export interface DesktopPagerProps extends Omit<React.ComponentProps<"nav">, "children"> {
  desktops: readonly DesktopPagerItem[];
  activeDesktopId: string;
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

function desktopLabel(control: DesktopControl, total: number, active: boolean): string {
  const base = `Desktop ${control.position + 1} of ${total}: ${control.desktop.name}`;
  return control.desktop.needsYou && !active ? `${base} — needs you` : base;
}

/**
 * Topbar desktop navigation (shell-rail v2 `.dots`): 6px dots, the active
 * desktop as an 18px pill, an orange dot for an off-screen desktop that needs
 * you. Dots only select; the ±2 overflow controls open the overview.
 */
export function DesktopPager({
  desktops,
  activeDesktopId,
  canSwitchDesktop = true,
  onSelectDesktop,
  onOpenOverview,
  className,
  "aria-label": ariaLabel = "Desktops",
  ...props
}: DesktopPagerProps) {
  const controls = pagerControls(desktops, activeDesktopId);
  const controlRefs = new Map<string, HTMLButtonElement>();
  const [rovingKey, setRovingKey] = useState<string>();
  const activeKey = `desktop:${activeDesktopId}`;
  const visibleKeys = new Set(controls.map(control => control.key));
  const tabStopKey = rovingKey && visibleKeys.has(rovingKey) ? rovingKey : activeKey;

  if (desktops.length === 0) return null;

  const focusControl = (position: number) => {
    const control = controls[position];
    if (!control) return;
    setRovingKey(control.key);
    controlRefs.get(control.key)?.focus();
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
      className={cn(
        "no-scrollbar mr-1 min-w-0 shrink overflow-x-auto overscroll-x-contain",
        className
      )}
      {...props}
    >
      <TooltipProvider>
        <ol className="flex w-max items-center" aria-label="Desktop positions">
          {controls.map(control => {
            const active = control.kind === "desktop" && control.desktop.id === activeDesktopId;
            const label =
              control.kind === "desktop"
                ? desktopLabel(control, desktops.length, active)
                : overflowLabel(control);
            const needsYou = control.kind === "desktop" && control.desktop.needsYou && !active;

            return (
              <li key={control.key}>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        ref={node => {
                          if (node) {
                            controlRefs.set(control.key, node);
                          } else {
                            controlRefs.delete(control.key);
                          }
                        }}
                        type="button"
                        variant="ghost"
                        aria-current={active ? "page" : undefined}
                        aria-disabled={
                          control.kind === "desktop" && !canSwitchDesktop ? true : undefined
                        }
                        aria-label={label}
                        data-active={active ? "true" : undefined}
                        data-needs-you={needsYou ? "true" : undefined}
                        data-direction={control.kind === "overflow" ? control.direction : undefined}
                        data-slot={
                          control.kind === "desktop"
                            ? "desktop-pager-control"
                            : "desktop-pager-overflow"
                        }
                        tabIndex={control.key === tabStopKey ? 0 : -1}
                        className={cn(
                          "h-7.5 w-6 min-w-0 rounded-sm p-0 hover:bg-rail-hover active:translate-y-0",
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
                      <span
                        aria-hidden="true"
                        className={cn(
                          "block h-1.5 rounded-pill transition-[width,background-color] duration-shell-base ease-spring motion-reduce:transition-none",
                          active && "w-4.5 bg-fg",
                          !active && "w-1.5",
                          !active &&
                            (needsYou ? "bg-attn" : "bg-indicator group-hover/button:bg-muted")
                        )}
                      />
                    ) : (
                      <Ellipsis aria-hidden="true" className="size-3.5 text-muted" />
                    )}
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    {control.kind === "desktop" ? control.desktop.name : label}
                  </TooltipContent>
                </Tooltip>
              </li>
            );
          })}
        </ol>
      </TooltipProvider>
    </nav>
  );
}
