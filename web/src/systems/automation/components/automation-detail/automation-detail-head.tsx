import type { LucideIcon } from "lucide-react";
import { Clock3, Pause, Radio, Webhook } from "lucide-react";
import type { ComponentProps } from "react";

import { Time, cn } from "@compozy/ui";

import { automationPauseLine, automationStartWord } from "../../lib/automation-detail";
import { automationLocationLabel, type AutomationView } from "../../lib/automation-view";
import type { AutomationStart } from "../../lib/automation-sentence";
import { AutomationEnableSwitch } from "../automation-enable-switch";

const START_ICONS = {
  schedule: Clock3,
  event: Radio,
  webhook: Webhook,
} as const satisfies Record<AutomationStart, LucideIcon>;

function DotSeparator() {
  return <span aria-hidden="true" className="mx-1.5 inline-block size-0.5 rounded-full bg-faint" />;
}

interface AutomationDetailHeadProps extends Omit<ComponentProps<"div">, "children"> {
  view: AutomationView;
  updatedAt: string;
  /** Most recent run start; events show it as "Last ran". */
  lastRanAt: string | null;
  isTogglePending: boolean;
  onToggleEnabled: (enabled: boolean) => void;
}

/**
 * Page head: the automation as one sentence with the switch that decides
 * whether it happens opposite it, then the facts that place and date it.
 */
export function AutomationDetailHead({
  view,
  updatedAt,
  lastRanAt,
  isTogglePending,
  onToggleEnabled,
  className,
  ...props
}: AutomationDetailHeadProps) {
  const StartIcon = START_ICONS[view.start];
  return (
    <div
      className={cn("mb-5.5 border-b border-line pb-4.5", className)}
      data-testid="automation-detail-head"
      {...props}
    >
      <div className="flex items-start justify-between gap-6">
        <div className="min-w-0 flex-1">
          <h1
            className="max-w-[46ch] text-pretty text-detail-h1 font-medium tracking-detail-h1 text-fg-2"
            data-testid="automation-detail-sentence"
          >
            {view.sentence.map((segment, index) =>
              segment.emphasis ? (
                <em className="text-fg-strong not-italic" key={`${index}-${segment.text}`}>
                  {segment.text}
                </em>
              ) : (
                <span key={`${index}-${segment.text}`}>{segment.text}</span>
              )
            )}
          </h1>
          {view.enabled ? null : (
            <p
              className="mt-1.5 flex max-w-[54ch] items-center gap-1.5 text-small-body text-muted"
              data-testid="automation-pause-line"
            >
              <Pause aria-hidden="true" className="size-3 shrink-0 text-subtle" />
              {automationPauseLine(view.start)}
            </p>
          )}
        </div>
        <AutomationEnableSwitch
          enabled={view.enabled}
          labelTestId="automation-enable-label"
          name={view.name}
          onEnabledChange={onToggleEnabled}
          pending={isTogglePending}
          switchTestId="automation-enable-switch"
        />
      </div>
      <div
        className="mt-3 flex flex-wrap items-center gap-y-1 text-eyebrow text-subtle"
        data-testid="automation-detail-subhead"
      >
        <span className="inline-flex items-center gap-1.5 text-muted">
          <StartIcon aria-hidden="true" className="size-3.5 text-subtle" />
          {automationStartWord(view.start)}
        </span>
        <DotSeparator />
        <span>
          {view.scope === "workspace" && (view.workspaceName ?? view.workspaceId) ? (
            <>
              Project{" "}
              <b className="font-medium text-fg">{view.workspaceName ?? view.workspaceId}</b>
            </>
          ) : (
            automationLocationLabel(view)
          )}
        </span>
        <DotSeparator />
        <SubheadTime lastRanAt={lastRanAt} view={view} />
        <DotSeparator />
        <span>
          Updated <Time className="text-fg-2" iso={updatedAt} />
        </span>
      </div>
    </div>
  );
}

/** Schedules say when they run next; events say when they last ran. */
function SubheadTime({ view, lastRanAt }: { view: AutomationView; lastRanAt: string | null }) {
  if (view.start === "schedule") {
    return view.enabled && view.nextRunAt ? (
      <span>
        Next run <Time className="text-fg-2" iso={view.nextRunAt} />
      </span>
    ) : (
      <span>No next run</span>
    );
  }
  return lastRanAt ? (
    <span>
      Last ran <Time className="text-fg-2" iso={lastRanAt} />
    </span>
  ) : (
    <span>Never ran</span>
  );
}
