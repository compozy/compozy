import { Text } from "lucide-react";

import { FieldDescription } from "@compozy/ui";

import { buildJobPreview } from "../../../lib/job-preview";
import {
  automationFormJobDraft,
  automationFormTriggerDraft,
  projectAutomationFormRequest,
  type AutomationFormDraft,
} from "../../../lib/automation-form-draft";
import { conditionFieldName } from "../../../lib/automation-form-events";
import type { AutomationSentence } from "../../../lib/automation-sentence";
import { buildTriggerPreview } from "../../../lib/trigger-preview";
import { AutomationSentenceText } from "../../automation-row-parts";
import { NextRunsCard } from "../../job-form/preview/next-runs-card";
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
        <SchedulePreview draft={draft} mode={mode} now={now} />
      ) : (
        <EventPreview draft={draft} mode={mode} />
      )}
      <AutomationRequestPayload request={request} />
    </div>
  );
}

function SchedulePreview({ draft, mode, now }: Omit<AutomationPreviewProps, "sentence">) {
  const preview = buildJobPreview(automationFormJobDraft(draft), now, mode);
  return (
    <NextRunsCard
      emptyReason={preview.nextRunsEmptyReason}
      nextRuns={preview.nextRuns?.slice(0, NEXT_RUNS_SHOWN) ?? null}
    />
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
