import { Button, Spinner, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";
import type { ProductUpdateIndicatorState } from "@/systems/settings";

/** What the shell hands the menubar: the indicator state and the in-place update. */
export interface MenubarUpdateModel {
  state: ProductUpdateIndicatorState;
  /** Absent when this install cannot update itself in place. */
  onUpdate?: () => void;
}

export interface MenubarUpdateIndicatorProps {
  state: ProductUpdateIndicatorState;
  /**
   * Starts the whole-product update in place. Absent when this install cannot
   * update itself (a package-manager install), so the control opens Settings,
   * where the upgrade command lives.
   */
  onUpdate?: () => void;
  /** Navigates to Settings → General, where the update's detail lives. */
  onOpenUpdates: () => void;
}

const OFFER_CLASS = "rounded-pill bg-brand-tint px-3 text-brand hover:bg-brand-tint-strong";
const FAILED_CLASS = "rounded-pill bg-destructive-tint px-3 text-destructive hover:opacity-90";
// The running update is disabled so it cannot be clicked twice, but it is still
// the offer pill mid-flight: keep the accent ink instead of Button's
// `disabled:text-subtle`, which reads as grey on the accent tint.
const UPDATING_CLASS = `${OFFER_CLASS} disabled:text-brand`;

/**
 * The menubar's one-click update.
 *
 * It exists only while there is something to say — an offer, the update running,
 * or a failure — and is removed from the DOM, not hidden with CSS, the rest of
 * the time. Clicking the offer updates CompozyOS in place: the daemon runs one
 * operation over every part of the product and the app restarts on the new
 * version by itself. The runtime/app split never reaches this control.
 */
export function MenubarUpdateIndicator({
  state,
  onUpdate,
  onOpenUpdates,
}: MenubarUpdateIndicatorProps) {
  if (state === "hidden") return null;

  if (state === "updating") {
    return (
      <Button
        className={UPDATING_CLASS}
        data-slot="os-menubar-update"
        data-state={state}
        data-testid="os-menubar-update"
        disabled
        size="sm"
        type="button"
        variant="ghost"
      >
        <Spinner aria-hidden="true" className="size-3.5" role="presentation" />
        <span role="status">Updating…</span>
      </Button>
    );
  }

  const failed = state === "failed";
  const onClick = onUpdate ?? onOpenUpdates;
  const label = failed ? (onUpdate ? "Retry update" : "Update failed") : "Update";
  const hint = failed
    ? onUpdate
      ? "Try the update again"
      : "See what went wrong"
    : onUpdate
      ? "Install the new version and restart"
      : "See how to update";

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            className={failed ? FAILED_CLASS : OFFER_CLASS}
            data-slot="os-menubar-update"
            data-state={state}
            data-testid="os-menubar-update"
            onClick={onClick}
            size="sm"
            type="button"
            variant="ghost"
          >
            {label}
          </Button>
        }
      />
      <TooltipContent side="bottom">{hint}</TooltipContent>
    </Tooltip>
  );
}
