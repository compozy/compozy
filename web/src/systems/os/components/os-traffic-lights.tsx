import { Maximize2, Minimize2, Minus, X, type LucideIcon } from "lucide-react";
import { Fragment } from "react";

import { Button } from "@compozy/ui";

import { cn } from "@/lib/utils";

/**
 * The window controls (shell-rail `.wctl`): trailing `quiet` icon buttons in
 * the order minimize, zoom, close — the window-head icon grammar:
 * `icon-sm` 26px pills with a 15px glyph, `muted` ink that turns `fg` on a
 * `surface-2` wash. A zoomed frame reads as a pressed toggle on the same
 * plate. They trail the deck row, or the head when the frame has no deck. The
 * owning window frame supplies each semantic action.
 *
 * Compact (<960px): the zoom control disappears (meaningless in a stack) and
 * interactive controls get non-overlapping 44px touch targets.
 */
export type OsTrafficLightAction = "close" | "minimize" | "zoom";

const ACTION_LABEL: Record<OsTrafficLightAction, string> = {
  close: "Close window",
  minimize: "Minimize window",
  zoom: "Zoom window",
};

const ACTION_ORDER: OsTrafficLightAction[] = ["minimize", "zoom", "close"];

const ACTION_ICON: Record<Exclude<OsTrafficLightAction, "zoom">, LucideIcon> = {
  close: X,
  minimize: Minus,
};

export interface OsTrafficLightsProps extends Omit<React.ComponentProps<"div">, "onSelect"> {
  /**
   * Called with the control's action when activated. When omitted the controls
   * render as non-interactive presentation (truthful chrome, no dead buttons).
   */
  onSelect?: (action: OsTrafficLightAction) => void;
  /** Compact presentation: zoom hidden, 44px hit areas. */
  compact?: boolean;
  /** Wraps the interactive zoom button (the zoom-menu hover anchor). */
  wrapZoom?: (button: React.ReactNode) => React.ReactNode;
  /** The frame currently fills its desktop; the zoom control reads as pressed. */
  zoomed?: boolean;
  /**
   * The surface the controls sit on. `strip` (the deck's recessed tab strip)
   * takes the chrome plate steps, which stay visible where `surface-2` would
   * vanish into the light strip.
   */
  surface?: "canvas" | "strip";
}

function Light({
  action,
  onSelect,
  compact,
  pressed,
  surface,
}: {
  action: OsTrafficLightAction;
  onSelect?: (action: OsTrafficLightAction) => void;
  compact: boolean;
  pressed?: boolean;
  surface: "canvas" | "strip";
}) {
  const label = action === "zoom" && pressed ? "Restore window" : ACTION_LABEL[action];
  const Icon = action === "zoom" ? (pressed ? Minimize2 : Maximize2) : ACTION_ICON[action];
  const glyph = <Icon aria-hidden="true" className="size-3.75" />;

  if (!onSelect) {
    return (
      <span
        aria-hidden="true"
        data-action={action}
        className={cn(
          "grid shrink-0 place-items-center rounded-pill text-muted",
          compact ? "size-11" : "size-button-icon-sm"
        )}
      >
        {glyph}
      </span>
    );
  }
  return (
    <Button
      type="button"
      variant="quiet"
      size="icon-sm"
      aria-label={label}
      title={label}
      aria-pressed={pressed}
      data-action={action}
      className={cn(
        surface === "strip"
          ? "hover:bg-rail-hover aria-pressed:bg-rail-selected aria-pressed:text-fg"
          : "aria-pressed:bg-surface-2 aria-pressed:text-fg",
        compact && "size-11 focus-visible:shadow-focus-inset"
      )}
      // The window frame owns pointer activation. Keep the native click, but
      // do not let its preceding mouse focus replace this button before click.
      onMouseDown={event => event.preventDefault()}
      onClick={() => onSelect(action)}
    >
      {glyph}
    </Button>
  );
}

export function OsTrafficLights({
  onSelect,
  compact = false,
  wrapZoom,
  zoomed = false,
  surface = "canvas",
  className,
  ...props
}: OsTrafficLightsProps) {
  const actions = compact ? ACTION_ORDER.filter(action => action !== "zoom") : ACTION_ORDER;
  return (
    <div
      data-slot="os-traffic-lights"
      data-presentation={compact ? "compact" : undefined}
      className={cn("flex shrink-0 items-center gap-0.5", className)}
      {...props}
    >
      {actions.map(action => {
        const light = (
          <Light
            action={action}
            onSelect={onSelect}
            compact={compact}
            pressed={action === "zoom" ? zoomed : undefined}
            surface={surface}
          />
        );
        if (action === "zoom" && wrapZoom && onSelect) {
          return <Fragment key={action}>{wrapZoom(light)}</Fragment>;
        }
        return <Fragment key={action}>{light}</Fragment>;
      })}
    </div>
  );
}
