import { Maximize2, Minimize2, Minus, X, type LucideIcon } from "lucide-react";
import { Fragment } from "react";

import { cn } from "@/lib/utils";

/**
 * The window controls, matching the shell-rail prototype (`.wctl`): quiet
 * trailing icon buttons in the order minimize, zoom, close — 28px targets
 * with a 15px glyph, subtle at rest and `fg` on a `surface-2` wash on hover.
 * They trail the deck row, or the head when the frame has no deck. The owning
 * window frame supplies each semantic action.
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
}

function Light({
  action,
  onSelect,
  compact,
  pressed,
}: {
  action: OsTrafficLightAction;
  onSelect?: (action: OsTrafficLightAction) => void;
  compact: boolean;
  pressed?: boolean;
}) {
  const label = action === "zoom" && pressed ? "Restore window" : ACTION_LABEL[action];
  const Icon = action === "zoom" ? (pressed ? Minimize2 : Maximize2) : ACTION_ICON[action];
  const box = cn(
    "grid shrink-0 place-items-center rounded-xs text-subtle",
    compact ? "size-11" : "size-7"
  );
  const glyph = <Icon aria-hidden="true" className="size-3.75" />;

  if (!onSelect) {
    return (
      <span aria-hidden="true" data-action={action} className={box}>
        {glyph}
      </span>
    );
  }
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      aria-pressed={pressed}
      data-action={action}
      className={cn(
        box,
        "transition-colors duration-fast hover:bg-surface-2 hover:text-fg focus-visible:outline-none",
        compact ? "focus-visible:shadow-focus-inset" : "focus-visible:shadow-focus-ring"
      )}
      // The window frame owns pointer activation. Keep the native click, but
      // do not let its preceding mouse focus replace this button before click.
      onMouseDown={event => event.preventDefault()}
      onClick={() => onSelect(action)}
    >
      {glyph}
    </button>
  );
}

export function OsTrafficLights({
  onSelect,
  compact = false,
  wrapZoom,
  zoomed = false,
  className,
  ...props
}: OsTrafficLightsProps) {
  const actions = compact ? ACTION_ORDER.filter(action => action !== "zoom") : ACTION_ORDER;
  return (
    <div
      data-slot="os-traffic-lights"
      data-presentation={compact ? "compact" : undefined}
      className={cn("flex shrink-0 items-center gap-px", className)}
      {...props}
    >
      {actions.map(action => {
        const light = (
          <Light
            action={action}
            onSelect={onSelect}
            compact={compact}
            pressed={action === "zoom" ? zoomed : undefined}
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
