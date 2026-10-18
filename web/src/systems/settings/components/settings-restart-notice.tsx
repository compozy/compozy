import { AlertTriangle, CheckCircle2 } from "lucide-react";

import { RestartBanner } from "@compozy/ui";

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

  const description = presentation.sessionLabel ? (
    <>
      {presentation.body}
      <span className="mt-0.5 block text-form-hint text-subtle">{presentation.sessionLabel}</span>
    </>
  ) : (
    presentation.body
  );

  return (
    <RestartBanner
      actionLabel={presentation.triggerLabel}
      actionProps={{
        "data-testid": `settings-page-${slug}-restart-trigger`,
        variant: "neutral",
      }}
      busy={presentation.phase === "polling"}
      className="rounded-md border-0 px-3.5 py-3 text-form-label md:px-3.5"
      data-testid={`settings-page-${slug}-restart-notice`}
      description={description}
      dismissLabel={presentation.dismissLabel}
      dismissProps={{ "data-testid": `settings-page-${slug}-restart-dismiss` }}
      icon={presentation.phase === "successful" ? <CheckCircle2 /> : <AlertTriangle />}
      isPending={presentation.triggerPending}
      message={presentation.title}
      onDismiss={presentation.dismissLabel ? restart.dismiss : undefined}
      pendingLabel={presentation.triggerLabel}
      restartNow={presentation.triggerLabel ? () => restart.trigger() : undefined}
      tone={presentation.tone}
    />
  );
}
