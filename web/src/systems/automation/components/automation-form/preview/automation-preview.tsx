import { Clock, Text } from "lucide-react";

import { FieldDescription } from "@compozy/ui";

import {
  automationFormTriggerDraft,
  projectAutomationFormRequest,
  type AutomationFormDraft,
} from "../../../lib/automation-form-draft";
import { conditionFieldName } from "../../../lib/automation-form-events";
import { scheduleNextRuns } from "../../../lib/automation-form-schedule";
import type { AutomationSentence } from "../../../lib/automation-sentence";
import { buildTriggerPreview } from "../../../lib/trigger-preview";
import { AutomationSentenceText } from "../../automation-row-parts";
import { AutomationNextRuns } from "../../automation-detail/automation-next-runs";
import { AutomationRequestPayload } from "./automation-request-payload";
import { PreviewCard } from "./preview-card";
import { RenderedPrompt } from "./rendered-prompt";
import { SampleEventCard } from "./sample-event-card";
import { WebhookEndpointCard } from "./webhook-endpoint-card";

const NEXT_RUNS_SHOWN = 4;

interface AutomationPreviewProps {
  draft: AutomationFormDraft;
  mode: "create" | "edit";
  now: number;
  sentence: AutomationSentence;
}

/**
 * What will happen before it does: the sentence, then the next runs for a
 * schedule or a sample event for events and links, then the exact request.
 */
export function AutomationPreview({ draft, mode, now, sentence }: AutomationPreviewProps) {
  const request = projectAutomationFormRequest(draft, mode);
  return (
    <div className="flex flex-col gap-3" data-testid="automation-preview">
      <PreviewCard icon={Text} label="Summary">
        <p className="text-small-body leading-relaxed text-muted">
          <AutomationSentenceText sentence={sentence} />
        </p>
      </PreviewCard>
      {draft.start === "schedule" ? (
        <SchedulePreview draft={draft} now={now} />
      ) : (
        <EventPreview draft={draft} mode={mode} />
      )}
      <AutomationRequestPayload request={request} />
    </div>
  );
}

const NEXT_RUNS_EMPTY_CLASS =
  "rounded-md border border-dashed border-line-soft bg-sunken px-3 py-2.5 text-form-hint leading-snug text-subtle";

function SchedulePreview({ draft, now }: Pick<AutomationPreviewProps, "draft" | "now">) {
  const { runs, emptyReason } = scheduleNextRuns(draft.schedule, now, NEXT_RUNS_SHOWN);
  return (
    <PreviewCard
      icon={Clock}
      label="Next runs"
      right={<span className="font-mono text-form-hint text-subtle">UTC</span>}
    >
      {runs === null ? (
        <div className={NEXT_RUNS_EMPTY_CLASS}>Choose Back to form to fix the schedule.</div>
      ) : runs.length === 0 ? (
        <div className={NEXT_RUNS_EMPTY_CLASS}>{emptyReason}</div>
      ) : (
        <AutomationNextRuns runs={runs} />
      )}
    </PreviewCard>
  );
}

function EventPreview({ draft, mode }: Pick<AutomationPreviewProps, "draft" | "mode">) {
  const preview = buildTriggerPreview(automationFormTriggerDraft(draft), { mode });
  const failing = preview.failingCondition;
  return (
    <>
      <SampleEventCard
        json={preview.json}
        matchLabel={preview.matchLabel}
        matchState={preview.matchState}
      >
        {failing ? (
          <FieldDescription className="mt-2">
            {conditionFieldName(failing.key)} must be{" "}
            <span className="font-mono text-fg">{failing.value}</span>.
          </FieldDescription>
        ) : null}
      </SampleEventCard>
      {preview.target.kind === "agent" ? (
        <RenderedPrompt rendered={preview.rendered} templateTokens={preview.templateTokens} />
      ) : null}
      {preview.webhook ? (
        <WebhookEndpointCard curl={preview.webhook.curl} url={preview.webhook.url} />
      ) : null}
    </>
  );
}
