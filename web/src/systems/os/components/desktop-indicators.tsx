import { Ellipsis } from "@compozy/ui/icons";
import type * as React from "react";

import { Button, Tooltip, TooltipContent, TooltipProvider, TooltipTrigger, cn } from "@compozy/ui";

import { useDesktopIndicatorRoving } from "../hooks/use-desktop-indicator-roving";
import { DESKTOP_DROP_TARGET_ATTRIBUTE } from "../hooks/use-os-tab-drag";
import {
  DESKTOP_OVERFLOW_ATTRIBUTE,
  desktopBarControls,
  desktopIndicatorLabel,
  desktopOverflowLabel,
  type DesktopBarControl,
  type DesktopIndicatorItem,
  type DesktopOverflowDirection,
  type DesktopOverflowRequest,
} from "../lib/desktop-indicator-controls";
import { DesktopIndicatorMenu } from "./desktop-indicator-menu";

export interface DesktopIndicatorsProps extends Omit<React.ComponentProps<"nav">, "children"> {
  desktops: readonly DesktopIndicatorItem[];
  activeDesktopId: string;
  /** Desktop switches and changes stay unavailable until the daemon has a live registered client. */
  canSwitchDesktop?: boolean;
  /** A tab is being dragged from this desktop: the indicators are drop targets. */
  dragSourceDesktopId?: string | null;
  /** The desktop the dragged tab would move to on release. */
  dropTargetDesktopId?: string | null;
  /** The overflow control the dragged tab rests on. */
  overflowHover?: DesktopOverflowDirection | null;
  onSelectDesktop: (desktopId: string) => void;
  onOpenOverview: (request: DesktopOverflowRequest) => void;
  onRenameDesktop: (desktopId: string) => void;
  onReorderDesktop: (desktopId: string, order: number) => void;
  onCloseDesktop: (desktopId: string) => void;
  onCreateDesktop: () => void;
}

function IndicatorDot({ active, needsYou }: { active: boolean; needsYou: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "block h-1.5 shrink-0 rounded-pill transition-[width,background-color] duration-shell-base ease-spring motion-reduce:transition-none",
        active && "w-4.5 bg-fg",
        !active && "w-1.5",
        !active && (needsYou ? "bg-attn" : "bg-indicator group-hover/button:bg-muted-foreground")
      )}
    />
  );
}

/**
 * Top-bar desktop indicators (shell-rail v2 `.dots`): 6px dots, the active desktop as an 18px
 * pill, an amber dot for an off-screen desktop that needs you. A click switches; right-click or
 * the menu keys open the desktop menu; past seven desktops, overflow controls on either side open
 * the overview. While a tab is dragged the indicators grow into labelled drop targets.
 */
export function DesktopIndicators({
  desktops,
  activeDesktopId,
  canSwitchDesktop = true,
  dragSourceDesktopId = null,
  dropTargetDesktopId = null,
  overflowHover = null,
  onSelectDesktop,
  onOpenOverview,
  onRenameDesktop,
  onReorderDesktop,
  onCloseDesktop,
  onCreateDesktop,
  className,
  "aria-label": ariaLabel = "Desktops",
  ...props
}: DesktopIndicatorsProps) {
  const controls = desktopBarControls(desktops, activeDesktopId);
  const { controlProps, rove } = useDesktopIndicatorRoving(controls, activeDesktopId);
  const dragging = dragSourceDesktopId !== null;

  if (desktops.length === 0) return null;

  const openOverflow = (control: Extract<DesktopBarControl, { kind: "overflow" }>) => {
    rove(control.key);
    onOpenOverview({
      direction: control.direction,
      hiddenDesktopIds: control.hiddenDesktopIds,
      anchorDesktopId: activeDesktopId,
    });
  };

  return (
    <nav
      aria-label={ariaLabel}
      data-slot="desktop-indicators"
      data-dragging={dragging ? "true" : undefined}
      className={cn(
        "no-scrollbar mr-1 min-w-0 shrink overflow-x-auto overscroll-x-contain",
        className
      )}
      {...props}
    >
      <TooltipProvider>
        <ol
          aria-label="Desktop positions"
          className={cn("flex w-max items-center", dragging && "gap-1")}
        >
          {controls.map(control => {
            if (control.kind === "overflow") {
              const label = desktopOverflowLabel(control);
              const hovered = overflowHover === control.direction;
              const overflowAttribute = { [DESKTOP_OVERFLOW_ATTRIBUTE]: control.direction };
              return (
                <li key={control.key}>
                  {dragging ? (
                    <button
                      {...controlProps(control)}
                      {...overflowAttribute}
                      aria-label={label}
                      className={cn(
                        "grid h-7.5 place-items-center rounded-sm border border-dashed border-line-strong px-2 text-muted-foreground",
                        hovered && "border-solid border-brand bg-brand-tint-strong text-fg"
                      )}
                      data-direction={control.direction}
                      data-slot="desktop-indicator-overflow"
                      type="button"
                      onClick={() => openOverflow(control)}
                    >
                      <span className="text-meta font-medium">
                        +{control.hiddenDesktopIds.length}
                      </span>
                    </button>
                  ) : (
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            {...controlProps(control)}
                            {...overflowAttribute}
                            aria-label={label}
                            className="h-7.5 w-6 min-w-0 rounded-sm p-0 hover:bg-rail-hover active:translate-y-0"
                            data-direction={control.direction}
                            data-slot="desktop-indicator-overflow"
                            type="button"
                            variant="ghost"
                            onClick={() => openOverflow(control)}
                          />
                        }
                      >
                        <Ellipsis aria-hidden="true" className="size-3.5 text-muted-foreground" />
                      </TooltipTrigger>
                      <TooltipContent side="bottom">{label}</TooltipContent>
                    </Tooltip>
                  )}
                </li>
              );
            }

            const { desktop } = control;
            const active = desktop.id === activeDesktopId;
            const needsYou = desktop.needsYou === true && !active;
            const label = desktopIndicatorLabel(control, desktops.length, active);
            const select = () => {
              rove(control.key);
              if (!canSwitchDesktop || active) return;
              onSelectDesktop(desktop.id);
            };
            const target = desktop.id === dropTargetDesktopId;
            return (
              <DesktopIndicatorMenu
                key={control.key}
                count={desktops.length}
                desktopId={desktop.id}
                disabled={!canSwitchDesktop || dragging}
                index={control.position}
                onClose={() => onCloseDesktop(desktop.id)}
                onCreate={onCreateDesktop}
                onRename={() => onRenameDesktop(desktop.id)}
                onReorder={order => onReorderDesktop(desktop.id, order)}
              >
                <li
                  {...{ [DESKTOP_DROP_TARGET_ATTRIBUTE]: desktop.id }}
                  className="group/indicator"
                >
                  {dragging ? (
                    <button
                      {...controlProps(control)}
                      aria-label={`Move tab to ${desktop.name}`}
                      className={cn(
                        "flex h-7 items-center gap-1.5 rounded-sm border border-dashed px-2 text-meta font-medium",
                        target
                          ? "border-solid border-brand bg-brand-tint-strong text-fg"
                          : "border-line-strong text-fg-2",
                        desktop.id === dragSourceDesktopId && !target && "opacity-50"
                      )}
                      data-drop-target={target ? "true" : undefined}
                      data-slot="desktop-indicator-target"
                      type="button"
                      onClick={select}
                    >
                      <IndicatorDot active={active} needsYou={needsYou} />
                      {desktop.name}
                    </button>
                  ) : (
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            {...controlProps(control)}
                            aria-current={active ? "page" : undefined}
                            aria-disabled={canSwitchDesktop ? undefined : true}
                            aria-label={label}
                            className={cn(
                              "h-7.5 w-6 min-w-0 rounded-sm p-0 hover:bg-rail-hover active:translate-y-0",
                              "group-data-popup-open/indicator:bg-rail-hover",
                              "aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
                            )}
                            data-active={active ? "true" : undefined}
                            data-needs-you={needsYou ? "true" : undefined}
                            data-slot="desktop-indicator"
                            type="button"
                            variant="ghost"
                            onClick={select}
                          />
                        }
                      >
                        <IndicatorDot active={active} needsYou={needsYou} />
                      </TooltipTrigger>
                      <TooltipContent side="bottom">{desktop.name}</TooltipContent>
                    </Tooltip>
                  )}
                </li>
              </DesktopIndicatorMenu>
            );
          })}
        </ol>
      </TooltipProvider>
    </nav>
  );
}
