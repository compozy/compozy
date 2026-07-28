import { Fragment } from "react";

import { cn } from "@/lib/utils";

/**
 * The three OS window controls, matching the OpenDesign prototype geometry:
 * 12px glyphs, 7px gap. Signal tones (accent/muted/success) appear only on
 * real buttons — never on inert presentation — and focus is a distinct ring,
 * not a restatement of hover. Interactive controls keep the 12px glyph inside
 * a ≥24px target. The owning window frame supplies each semantic action.
 *
 * Compact (<960px, os-v2.css mobile block): the zoom control disappears
 * (meaningless in a stack), glyphs grow to 15px, and interactive controls get
 * non-overlapping 44px touch targets. Inert chrome uses the visual 12px gap.
 */
export type OsTrafficLightAction = "close" | "minimize" | "zoom";

const ACTION_LABEL: Record<OsTrafficLightAction, string> = {
  close: "Close window",
  minimize: "Minimize window",
  zoom: "Zoom window",
};

const ACTION_ORDER: OsTrafficLightAction[] = ["close", "minimize", "zoom"];

// Tone applies to the 12px glyph on button hover/focus, per the prototype.
const ACTION_TONE: Record<OsTrafficLightAction, string> = {
  close:
    "group-hover/traffic:bg-accent group-hover/traffic:border-accent group-focus-visible/traffic:bg-accent group-focus-visible/traffic:border-accent",
  minimize:
    "group-hover/traffic:bg-muted group-hover/traffic:border-muted group-focus-visible/traffic:bg-muted group-focus-visible/traffic:border-muted",
  zoom: "group-hover/traffic:bg-success group-hover/traffic:border-success group-focus-visible/traffic:bg-success group-focus-visible/traffic:border-success",
};

export interface OsTrafficLightsProps extends Omit<React.ComponentProps<"div">, "onSelect"> {
  /**
   * Called with the control's action when activated. When omitted the controls
   * render as non-interactive presentation (truthful chrome, no dead buttons).
   */
  onSelect?: (action: OsTrafficLightAction) => void;
  /** Compact presentation: zoom hidden, enlarged glyphs and hit areas. */
  compact?: boolean;
  /** Wraps the interactive zoom button (the zoom-menu hover anchor). */
  wrapZoom?: (button: React.ReactNode) => React.ReactNode;
}

function Light({
  action,
  onSelect,
  compact,
}: {
  action: OsTrafficLightAction;
  onSelect?: (action: OsTrafficLightAction) => void;
  compact: boolean;
}) {
  const glyph = cn(
    "rounded-xs border border-line-strong bg-btn-default-fill transition-colors duration-base",
    compact ? "size-traffic-light-compact" : "size-traffic-light"
  );

  if (!onSelect) {
    return <span aria-hidden="true" data-action={action} className={glyph} />;
  }
  return (
    <button
      type="button"
      aria-label={ACTION_LABEL[action]}
      data-action={action}
      className={cn(
        "group/traffic grid place-items-center rounded-xs focus-visible:outline-none",
        compact
          ? "size-traffic-light-compact-target focus-visible:shadow-focus-inset"
          : // ≥24px target around the 12px glyph; -mx-1.5 (-6px each side) so
            // adjacent button starts are 19px apart and the glyph edge gap is 7px.
            "-mx-1.5 size-6 focus-visible:shadow-focus-ring"
      )}
      // The window frame owns pointer activation. Keep the native click, but
      // do not let its preceding mouse focus replace this button before click.
      onMouseDown={event => event.preventDefault()}
      onClick={() => onSelect(action)}
    >
      <span aria-hidden="true" className={cn(glyph, ACTION_TONE[action])} />
    </button>
  );
}

export function OsTrafficLights({
  onSelect,
  compact = false,
  wrapZoom,
  className,
  ...props
}: OsTrafficLightsProps) {
  const actions = compact ? ACTION_ORDER.filter(action => action !== "zoom") : ACTION_ORDER;
  return (
    <div
      data-slot="os-traffic-lights"
      data-presentation={compact ? "compact" : undefined}
      className={cn(
        "flex items-center",
        compact ? (onSelect ? "gap-0" : "gap-traffic-light-compact-gap") : "gap-traffic-light-gap",
        className
      )}
      {...props}
    >
      {actions.map(action => {
        const light = <Light action={action} onSelect={onSelect} compact={compact} />;
        if (action === "zoom" && wrapZoom && onSelect) {
          return <Fragment key={action}>{wrapZoom(light)}</Fragment>;
        }
        return <Fragment key={action}>{light}</Fragment>;
      })}
    </div>
  );
}
