import { AlertTriangle, CheckCircle2 } from "lucide-react";

import { Button, cn, Spinner } from "@agh/ui";

import {
  settingsRestartPresentation,
  type SettingsRestartViewState,
} from "../lib/restart-presentation";

type RestartState = SettingsRestartViewState;

/**
 * Inline restart notice (design system §04): states what happens if you don't
 * act, and offers Restart / Not now without leaving the current page.
 */
export function SettingsRestartNotice({ restart, slug }: { restart: RestartState; slug: string }) {
  const presentation = settingsRestartPresentation(restart);
  if (!presentation) return null;

  const toneClasses = {
    danger: { surface: "bg-danger-tint", icon: "text-danger" },
    info: { surface: "bg-info-tint", icon: "text-info" },
    success: { surface: "bg-success-tint", icon: "text-success" },
    warning: { surface: "bg-warning-tint", icon: "text-warning" },
  }[presentation.tone];

  return (
    <div
      className={cn(
        "flex items-start gap-2.5 rounded-md px-3.5 py-3 text-form-label leading-normal text-muted",
        toneClasses.surface
      )}
      data-testid={`settings-page-${slug}-restart-notice`}
      role={presentation.role}
    >
      {presentation.phase === "polling" ? (
        <Spinner className={cn("mt-0.5 size-3.5 shrink-0", toneClasses.icon)} />
      ) : presentation.phase === "successful" ? (
        <CheckCircle2
          aria-hidden="true"
          className={cn("mt-0.5 size-3.5 shrink-0", toneClasses.icon)}
        />
      ) : (
        <AlertTriangle
          aria-hidden="true"
          className={cn("mt-0.5 size-3.5 shrink-0", toneClasses.icon)}
        />
      )}
      <div className="min-w-0 flex-1">
        <span className="block text-ws-name font-medium text-fg-strong">{presentation.title}</span>
        {presentation.body}
        {presentation.sessionLabel ? (
          <span className="mt-0.5 block text-form-hint text-subtle">
            {presentation.sessionLabel}
          </span>
        ) : null}
      </div>
      {presentation.dismissLabel && !presentation.triggerLabel ? (
        <div className="ml-2 flex shrink-0 items-center">
          <Button
            data-testid={`settings-page-${slug}-restart-dismiss`}
            onClick={restart.dismiss}
            size="sm"
            type="button"
            variant="ghost"
          >
            {presentation.dismissLabel}
          </Button>
        </div>
      ) : presentation.triggerLabel ? (
        <div className="ml-2 flex shrink-0 items-center gap-1.5">
          {presentation.dismissLabel ? (
            <Button
              data-testid={`settings-page-${slug}-restart-dismiss`}
              onClick={restart.dismiss}
              size="sm"
              type="button"
              variant="ghost"
            >
              {presentation.dismissLabel}
            </Button>
          ) : null}
          <Button
            aria-busy={presentation.triggerPending || undefined}
            data-testid={`settings-page-${slug}-restart-trigger`}
            disabled={presentation.triggerPending}
            onClick={() => restart.trigger()}
            size="sm"
            type="button"
            variant="neutral"
          >
            {presentation.triggerPending ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {presentation.triggerLabel}
          </Button>
        </div>
      ) : null}
    </div>
  );
}
