import { useRef } from "react";

import { Field, FieldDescription, FieldError, FieldLabel, Textarea } from "@compozy/ui";

import type { AutomationFormModel } from "../../../hooks/use-automation-form";
import { detailChipsFor } from "../../../lib/automation-form-events";
import type { AutomationStart } from "../../../lib/automation-sentence";
import { ChoiceChip } from "../choice-chip";
import { AgentCommandSelect, type AgentPayload } from "@/systems/agent";

interface AgentTargetProps {
  agent: string;
  agentLocked: boolean;
  agents: AgentPayload[];
  agentsError?: string | null;
  agentsLoading?: boolean;
  form: AutomationFormModel;
  prompt: string;
  start: AutomationStart;
}

/** Ask an agent: the agent and the message. Event and link messages can insert event details. */
export function AgentTarget({
  agent,
  agentLocked,
  agents,
  agentsError = null,
  agentsLoading = false,
  form,
  prompt,
  start,
}: AgentTargetProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const chips = start === "schedule" ? [] : detailChipsFor(form.eventDef);

  const insert = (value: string) => {
    const textarea = textareaRef.current;
    const from = textarea?.selectionStart ?? prompt.length;
    const to = textarea?.selectionEnd ?? prompt.length;
    form.onPromptChange(`${prompt.slice(0, from)}${value}${prompt.slice(to)}`);
    const caret = from + value.length;
    // Restore the caret after React commits the controlled value.
    requestAnimationFrame(() => {
      textarea?.focus();
      textarea?.setSelectionRange(caret, caret);
    });
  };

  return (
    <div className="flex flex-col gap-4">
      <Field data-invalid={form.agentMissing || undefined}>
        <FieldLabel htmlFor="automation-agent">Agent</FieldLabel>
        <AgentCommandSelect
          agents={agents}
          disabled={agentLocked}
          error={agentsError}
          loading={agentsLoading}
          onChange={next => form.onAgentChange(next ?? "")}
          triggerId="automation-agent"
          triggerTestId="automation-agent-input"
          value={agent || null}
        />
        {form.agentMissing ? (
          <FieldError>Choose which agent should get the message.</FieldError>
        ) : null}
      </Field>
      <Field>
        <FieldLabel htmlFor="automation-prompt">Message</FieldLabel>
        <Textarea
          className="min-h-24"
          data-testid="automation-prompt-input"
          id="automation-prompt"
          onChange={event => form.onPromptChange(event.target.value)}
          ref={textareaRef}
          value={prompt}
        />
        {start === "schedule" ? (
          <FieldDescription>Sent as written.</FieldDescription>
        ) : (
          <div className="flex flex-col gap-1.5">
            <FieldDescription>Add details from the event:</FieldDescription>
            <div aria-label="Event details" className="flex flex-wrap gap-1.5" role="group">
              {chips.map(chip => (
                <ChoiceChip
                  aria-label={`Insert ${chip.label}`}
                  className="bg-info-tint font-mono text-mono-id text-fg"
                  key={chip.value}
                  onClick={() => insert(chip.value)}
                >
                  {chip.label}
                </ChoiceChip>
              ))}
            </div>
          </div>
        )}
      </Field>
    </div>
  );
}
