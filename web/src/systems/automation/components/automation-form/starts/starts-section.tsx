import { Clock3, Radio, Webhook } from "lucide-react";

import type { AutomationFormDraft } from "../../../lib/automation-form-draft";
import { EDITOR_EVENT_CARDS } from "../../../lib/automation-form-events";
import { describeSchedule, type AutomationStart } from "../../../lib/automation-sentence";
import { parseEventSelection } from "../../../lib/trigger-event-id";
import { AutomationChoiceCards, type AutomationChoice } from "../automation-choice-cards";
import { AutomationFormSection } from "../automation-form-section";
import type { AutomationFormModel } from "../../../hooks/use-automation-form";
import { EventStart } from "./event-start";
import { ScheduleStart } from "./schedule-start";
import { WebhookStart } from "./webhook-start";

const START_CHOICES: readonly AutomationChoice<AutomationStart>[] = [
  {
    value: "schedule",
    icon: Clock3,
    title: "On a schedule",
    description: "Every morning, every hour, or once at a set time.",
  },
  {
    value: "event",
    icon: Radio,
    title: "When something happens",
    description: "A session starts or stops, a hook finishes, an extension sends an event.",
  },
  {
    value: "webhook",
    icon: Webhook,
    title: "When another app calls a link",
    description: "GitHub, CI or any service sends a signed request.",
  },
];

/** The chosen start's settings in a few words, shown on its card while editing. */
function startSummary(draft: AutomationFormDraft, timeZone: string | undefined): string {
  if (draft.start === "schedule") return describeSchedule(draft.schedule, { timeZone });
  if (draft.start === "webhook") {
    const slug = draft.endpoint_slug?.trim();
    return slug ? `The ${slug} link` : "A link another app calls";
  }
  const catalogId = parseEventSelection(draft.event).catalogId;
  return EDITOR_EVENT_CARDS.find(card => card.id === catalogId)?.label ?? draft.event;
}

interface StartsSectionProps {
  draft: AutomationFormDraft;
  form: AutomationFormModel;
  mode: "create" | "edit";
}

/** 02 Starts: three start cards decide the entity; the chosen one opens its settings. */
export function StartsSection({ draft, form, mode }: StartsSectionProps) {
  const locked = mode === "edit";
  const choices = START_CHOICES.map(choice =>
    !locked
      ? choice
      : choice.value === draft.start
        ? { ...choice, description: startSummary(draft, form.timeZone) }
        : { ...choice, lockedReason: "Locked" }
  );

  return (
    <AutomationFormSection
      data-testid="automation-form-starts"
      description={
        locked
          ? "Can't change after creating. Make a new automation instead."
          : "When should it run?"
      }
      number={2}
      title="Starts"
    >
      <AutomationChoiceCards
        choices={choices}
        label="When it starts"
        onChange={form.onStart}
        testIdPrefix="automation-start"
        value={draft.start}
      />
      <div
        className="rounded-lg bg-sunken p-3.5"
        data-testid={`automation-start-inset-${draft.start}`}
      >
        {draft.start === "schedule" ? <ScheduleStart draft={draft} form={form} /> : null}
        {draft.start === "event" ? <EventStart form={form} /> : null}
        {draft.start === "webhook" ? <WebhookStart draft={draft} form={form} /> : null}
      </div>
    </AutomationFormSection>
  );
}
