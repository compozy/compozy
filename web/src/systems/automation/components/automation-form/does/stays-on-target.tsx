import {
  Alert,
  AlertDescription,
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  NativeSelect,
  NativeSelectOption,
  Textarea,
} from "@compozy/ui";

import type { AutomationFormModel } from "../../../hooks/use-automation-form";
import {
  staysOnTargetModel,
  type StaysOnTargetStatus,
} from "../../../lib/automation-stays-on-target";
import { useAgentInstancesState } from "@/systems/agent";
import { useProfileReadScope } from "@/systems/profiles";

interface StaysOnTargetProps {
  agent: string;
  prompt: string;
  /** The trigger's own scope: `""` for User. */
  workspaceId: string;
  locked: boolean;
  form: AutomationFormModel;
}

function StatusNote({ status }: { status: StaysOnTargetStatus }) {
  switch (status.kind) {
    case "missing":
      return (
        <FieldError data-testid="automation-stays-on-missing">
          {status.agent} isn't running in {status.scope}. Start it there, or pick where it runs.
        </FieldError>
      );
    case "paused":
      return (
        <FieldDescription data-testid="automation-stays-on-paused">
          {status.label} is paused. Events wait for its next check after you resume it.
        </FieldDescription>
      );
    case "no-checks":
      return (
        <FieldDescription data-testid="automation-stays-on-no-checks">
          {status.label} has check-ins off, so no check will read these events.
        </FieldDescription>
      );
    default:
      return null;
  }
}

/**
 * Tell a stays-on agent (S10): the event waits for the agent's next check, which reads it and
 * proposes what to do. Nothing runs on its own; unattended work stays with Ask an agent.
 */
export function StaysOnTarget({ agent, prompt, workspaceId, locked, form }: StaysOnTargetProps) {
  const { instances, settled } = useAgentInstancesState();
  const { destination } = useProfileReadScope();
  const profile = form.profile || destination;
  const model = staysOnTargetModel({
    instances: instances.filter(instance => instance.profile_name === profile),
    agent,
    workspaceId,
  });
  const missing = model.status.kind === "missing";
  return (
    <div className="flex flex-col gap-4" data-testid="automation-stays-on-target">
      <Alert variant="info">
        <AlertDescription>
          The agent reads the event at its next check and proposes what to do. It doesn't act on its
          own. For work that should run unattended, choose Ask an agent.
        </AlertDescription>
      </Alert>
      <div className="grid grid-cols-2 gap-3 max-[560px]:grid-cols-1">
        <Field>
          <FieldLabel htmlFor="automation-stays-on-agent">Stays-on agent</FieldLabel>
          <NativeSelect
            data-testid="automation-stays-on-agent"
            disabled={locked || model.agents.length === 0}
            id="automation-stays-on-agent"
            onChange={event => form.onAgentChange(event.target.value)}
            value={agent}
          >
            <NativeSelectOption value="">
              {!settled
                ? "Loading agents…"
                : model.agents.length === 0
                  ? "No stays-on agents yet"
                  : "Choose an agent…"}
            </NativeSelectOption>
            {model.agents.map(option => (
              <NativeSelectOption key={option.name} value={option.name}>
                {option.label}
              </NativeSelectOption>
            ))}
            {agent && !model.agents.some(option => option.name === agent) ? (
              <NativeSelectOption value={agent}>{agent}</NativeSelectOption>
            ) : null}
          </NativeSelect>
        </Field>
        <Field data-invalid={missing || undefined}>
          <FieldLabel htmlFor="automation-stays-on-scope">Where it runs</FieldLabel>
          <NativeSelect
            data-testid="automation-stays-on-scope"
            disabled={locked || model.scopes.length === 0}
            id="automation-stays-on-scope"
            onChange={event => form.onInstanceScope(event.target.value)}
            value={model.selectedScope ?? "@none"}
          >
            {model.selectedScope === null ? (
              <NativeSelectOption disabled value="@none">
                Choose where…
              </NativeSelectOption>
            ) : null}
            {model.scopes.map(scope => (
              <NativeSelectOption key={scope.workspaceId || "@global"} value={scope.workspaceId}>
                {scope.label}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </Field>
      </div>
      {settled ? <StatusNote status={model.status} /> : null}
      <Field>
        <FieldLabel htmlFor="automation-stays-on-prompt">What to look for</FieldLabel>
        <Textarea
          className="min-h-20"
          data-testid="automation-stays-on-prompt"
          id="automation-stays-on-prompt"
          onChange={event => form.onPromptChange(event.target.value)}
          value={prompt}
        />
        <FieldDescription>
          The agent reads this with the event. The event itself is only data.
        </FieldDescription>
      </Field>
    </div>
  );
}
