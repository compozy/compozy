import { useId, type ComponentProps } from "react";

import { Pill, Switch, Time, cn } from "@compozy/ui";

import { buildTriggerLede, triggerEventLabel, triggerPauseLine } from "../../lib/trigger-sentence";
import type { AutomationTrigger } from "../../types";

interface TriggerEnableSwitchProps extends Omit<ComponentProps<"div">, "children"> {
  enabled: boolean;
  pending: boolean;
  onEnabledChange: (enabled: boolean) => void;
}

/**
 * The labeled enable switch — the only accent on the page.
 *
 * While the PATCH is in flight the track keeps the state the daemon last
 * confirmed; the label announces the transition instead. An optimistic flip
 * would claim a state the runtime has not agreed to yet.
 */
function TriggerEnableSwitch({
  enabled,
  pending,
  onEnabledChange,
  className,
  ...props
}: TriggerEnableSwitchProps) {
  const labelId = useId();
  const label = pending ? (enabled ? "Disabling…" : "Enabling…") : enabled ? "Enabled" : "Disabled";
  return (
    <div
      aria-busy={pending || undefined}
      className={cn(
        "mt-0.5 flex shrink-0 items-center gap-2 rounded-md px-1.5 py-1",
        pending && "pointer-events-none opacity-55",
        className
      )}
      {...props}
    >
      <span
        className={cn(
          "text-form-label font-medium transition-colors duration-fast ease-out",
          enabled ? "text-fg" : "text-muted"
        )}
        data-testid="trigger-enable-label"
        id={labelId}
      >
        {label}
      </span>
      <Switch
        aria-labelledby={labelId}
        checked={enabled}
        data-testid="trigger-enable-switch"
        disabled={pending}
        onCheckedChange={next => {
          if (!pending) onEnabledChange(next);
        }}
      />
    </div>
  );
}

const DOT_SEPARATOR = (
  <span aria-hidden="true" className="mx-1.5 inline-block size-0.5 rounded-full bg-faint" />
);

interface TriggerDetailHeadProps extends Omit<ComponentProps<"div">, "children"> {
  trigger: AutomationTrigger;
  workspaceName: string | null;
  /** Most recent run start from the loaded run sample; omitted when unknown. */
  lastRanAt: string | null;
  isTogglePending: boolean;
  onToggleEnabled: (enabled: boolean) => void;
}

/**
 * Page head: the rule as one sentence, with the control that decides whether it
 * happens opposite it, then the facts that date the record.
 */
export function TriggerDetailHead({
  trigger,
  workspaceName,
  lastRanAt,
  isTogglePending,
  onToggleEnabled,
  className,
  ...props
}: TriggerDetailHeadProps) {
  const segments = buildTriggerLede(trigger, workspaceName);
  return (
    <div
      className={cn("mb-5.5 border-b border-line pb-4.5", className)}
      data-testid="trigger-detail-head"
      {...props}
    >
      <div className="flex items-start justify-between gap-6">
        <div className="min-w-0 flex-1">
          <h1
            className="max-w-[46ch] text-pretty text-empty-h1 font-medium leading-snug tracking-empty-h1 text-fg-strong"
            data-testid="trigger-detail-sentence"
          >
            {segments.map((segment, index) =>
              segment.em ? (
                <em className="text-fg not-italic" key={`${index}-${segment.text}`}>
                  {segment.text}
                </em>
              ) : (
                <span key={`${index}-${segment.text}`}>{segment.text}</span>
              )
            )}
          </h1>
          {trigger.enabled ? null : (
            <p
              className="mt-1.5 max-w-[54ch] text-small-body text-muted"
              data-testid="trigger-pause-line"
            >
              {triggerPauseLine(trigger)}
            </p>
          )}
        </div>
        <TriggerEnableSwitch
          enabled={trigger.enabled}
          onEnabledChange={onToggleEnabled}
          pending={isTogglePending}
        />
      </div>
      <div
        className="mt-3 flex flex-wrap items-center gap-y-1 text-form-label text-subtle"
        data-testid="trigger-detail-subhead"
      >
        <Pill className="mr-2" size="sm" tone="info">
          {triggerEventLabel(trigger)}
        </Pill>
        <span>
          {trigger.scope === "workspace" ? (
            <>
              Workspace{" "}
              <b className="font-medium text-muted">
                {workspaceName ?? trigger.workspace_id ?? ""}
              </b>
            </>
          ) : (
            "Every workspace"
          )}
        </span>
        {lastRanAt ? (
          <>
            {DOT_SEPARATOR}
            <span>
              Last ran <Time className="text-muted" iso={lastRanAt} />
            </span>
          </>
        ) : null}
        {DOT_SEPARATOR}
        <span>
          Updated <Time className="text-muted" iso={trigger.updated_at} />
        </span>
      </div>
    </div>
  );
}
