import { Anchor, CirclePlay, CircleStop, Puzzle, type LucideIcon } from "lucide-react";

import { Field, FieldDescription, FieldLabel, Input, RadioCard } from "@compozy/ui";

import type { AutomationFormModel } from "../../../hooks/use-automation-form";
import { EDITOR_EVENT_CARDS, type EditorEventCardId } from "../../../lib/automation-form-events";

const EVENT_ICONS: Record<EditorEventCardId, LucideIcon> = {
  "session.created": CirclePlay,
  "session.stopped": CircleStop,
  "hook.completed": Anchor,
  ext: Puzzle,
};

const MONO = "font-mono text-form-label";

/** Event settings: the four events CompozyOS emits, plus the hook or extension name. */
export function EventStart({ form }: { form: AutomationFormModel }) {
  const { selection } = form;
  return (
    <div className="flex flex-col gap-3">
      <div
        aria-label="What happens"
        className="grid grid-cols-1 gap-2 sm:grid-cols-2"
        role="radiogroup"
      >
        {EDITOR_EVENT_CARDS.map(card => (
          <RadioCard
            data-testid={`automation-event-${card.id}`}
            description={card.description}
            icon={EVENT_ICONS[card.id]}
            iconWellSize="lg"
            key={card.id}
            onSelect={() => form.onEventCard(card.id)}
            selected={selection.catalogId === card.id}
            title={card.label}
          />
        ))}
      </div>
      {selection.family === "hook" ? (
        <Field>
          <FieldLabel htmlFor="automation-hook-name">Hook name</FieldLabel>
          <Input
            className={MONO}
            data-testid="automation-hook-name"
            id="automation-hook-name"
            onChange={event => form.onHookName(event.target.value)}
            placeholder="transform"
            value={selection.hookName}
          />
          <FieldDescription>
            Runs when <code className="font-mono text-mono-id">hook.&lt;name&gt;.completed</code>{" "}
            fires.
          </FieldDescription>
        </Field>
      ) : null}
      {selection.family === "ext" ? (
        <div className="grid grid-cols-2 gap-3">
          <Field>
            <FieldLabel htmlFor="automation-ext-name">Extension</FieldLabel>
            <Input
              className={MONO}
              data-testid="automation-ext-name"
              id="automation-ext-name"
              onChange={event => form.onExtension(event.target.value, selection.extEvent)}
              placeholder="release"
              value={selection.extExt}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="automation-ext-event">Event</FieldLabel>
            <Input
              className={MONO}
              data-testid="automation-ext-event"
              id="automation-ext-event"
              onChange={event => form.onExtension(selection.extExt, event.target.value)}
              placeholder="deploy-started"
              value={selection.extEvent}
            />
          </Field>
        </div>
      ) : null}
    </div>
  );
}
