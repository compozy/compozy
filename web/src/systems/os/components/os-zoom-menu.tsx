import type { ReactNode } from "react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@agh/ui";

import { cn } from "@/lib/utils";

import { useOsZoomMenu } from "../hooks/use-os-zoom-menu";
import {
  WINDOW_ARRANGE_COMMANDS,
  WINDOW_PLACEMENT_COMMANDS,
  type WindowPlacementId,
} from "../lib/window-manager-command-registry";

/**
 * macOS-style zoom-button menu (Sequoia green-button posture): hovering the
 * zoom traffic light opens Move & Resize (halves + quarters as zone glyphs,
 * plus Fill & Arrange (fill, 2-up, grid). Click stays zoom; every action here
 * also lives in the palette — the guaranteed
 * keyboard path — so the menu is discoverability, never the only route.
 * The hidden trigger span only anchors the Radix content; hover intent lives
 * on the wrapper so the real button keeps its own semantics.
 */

interface GlyphZone {
  x: number;
  y: number;
  w: number;
  h: number;
}

const GLYPH_ZONES: Record<WindowPlacementId, GlyphZone> = {
  left: { x: 0, y: 0, w: 0.5, h: 1 },
  right: { x: 0.5, y: 0, w: 0.5, h: 1 },
  top: { x: 0, y: 0, w: 1, h: 0.5 },
  bottom: { x: 0, y: 0.5, w: 1, h: 0.5 },
  "top-left": { x: 0, y: 0, w: 0.5, h: 0.5 },
  "top-right": { x: 0.5, y: 0, w: 0.5, h: 0.5 },
  "bottom-left": { x: 0, y: 0.5, w: 0.5, h: 0.5 },
  "bottom-right": { x: 0.5, y: 0.5, w: 0.5, h: 0.5 },
};

function ZoneGlyph({ zones, className }: { zones: readonly GlyphZone[]; className?: string }) {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 16 12"
      className={cn("size-4 text-current", className)}
      fill="none"
    >
      <rect
        x="0.5"
        y="0.5"
        width="15"
        height="11"
        rx="2"
        stroke="currentColor"
        strokeOpacity="0.55"
      />
      {zones.map((zone, index) => (
        <rect
          key={index}
          x={1.5 + 13 * zone.x}
          y={1.5 + 9 * zone.y}
          width={Math.max(13 * zone.w - 1, 1.5)}
          height={Math.max(9 * zone.h - 1, 1.5)}
          rx="0.75"
          fill="currentColor"
          fillOpacity={index === 0 ? 0.85 : 0.4}
        />
      ))}
    </svg>
  );
}

export interface OsZoomMenuProps {
  windowId: string;
  /** The real zoom traffic-light button (keeps its own click = toggleZoom). */
  children: ReactNode;
}

export function OsZoomMenu({ windowId, children }: OsZoomMenuProps) {
  const menu = useOsZoomMenu(windowId);
  return (
    <DropdownMenu open={menu.open} onOpenChange={menu.onOpenChange} modal={false}>
      <span
        data-slot="os-zoom-menu-anchor"
        className="relative inline-flex"
        onPointerEnter={menu.onHoverEnter}
        onPointerLeave={menu.onHoverLeave}
      >
        {children}
        <DropdownMenuTrigger
          nativeButton={false}
          render={
            <span
              aria-hidden="true"
              tabIndex={-1}
              className="pointer-events-none absolute inset-0"
            />
          }
        />
      </span>
      <DropdownMenuContent
        align="start"
        sideOffset={6}
        data-testid="os-zoom-menu"
        onPointerEnter={menu.onContentEnter}
        onPointerLeave={menu.onHoverLeave}
      >
        <DropdownMenuGroup>
          <DropdownMenuLabel>Move &amp; Resize</DropdownMenuLabel>
          <div className="grid grid-cols-4">
            {WINDOW_PLACEMENT_COMMANDS.map(command => {
              const zoneId = command.placement;
              return (
                <DropdownMenuItem
                  key={command.id}
                  aria-label={command.label}
                  title={command.label}
                  data-testid={`os-zoom-menu-${zoneId}`}
                  className="size-11 justify-center p-0"
                  disabled={!menu.placementEnabled}
                  onClick={() => menu.dispatchPlacement(command)}
                >
                  <ZoneGlyph zones={[GLYPH_ZONES[zoneId]]} />
                </DropdownMenuItem>
              );
            })}
          </div>
          {!menu.floating ? (
            <DropdownMenuItem
              data-testid="os-zoom-menu-make-floating"
              className="min-h-11"
              onClick={menu.dispatchMakeFloating}
            >
              Make window floating
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuGroup>
          <DropdownMenuLabel>Fill &amp; Arrange</DropdownMenuLabel>
          <div className="grid grid-cols-4">
            <DropdownMenuItem
              aria-label="Fill window"
              title="Fill window"
              data-testid="os-zoom-menu-fill"
              className="size-11 justify-center p-0"
              onClick={() => menu.dispatchFill()}
            >
              <ZoneGlyph zones={[{ x: 0, y: 0, w: 1, h: 1 }]} />
            </DropdownMenuItem>
            {WINDOW_ARRANGE_COMMANDS.map(command => (
              <DropdownMenuItem
                key={command.preset}
                aria-label={command.label}
                title={command.label}
                data-testid={`os-zoom-menu-${command.preset}`}
                className="size-11 justify-center p-0"
                disabled={!menu.arrangeEnabled}
                onClick={() => menu.dispatchArrange(command.preset)}
              >
                <ZoneGlyph
                  zones={
                    command.preset === "two-up"
                      ? [GLYPH_ZONES.left, GLYPH_ZONES.right]
                      : [
                          GLYPH_ZONES["top-left"],
                          GLYPH_ZONES["top-right"],
                          GLYPH_ZONES["bottom-left"],
                          GLYPH_ZONES["bottom-right"],
                        ]
                  }
                />
              </DropdownMenuItem>
            ))}
          </div>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
