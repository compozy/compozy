import { Check } from "@compozy/ui/icons";

import {
  Button,
  Field,
  FieldError,
  FieldLabel,
  Input,
  MarkdownEditor,
  StreamMarkdown,
  cn,
} from "@compozy/ui";

import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";

import type { AgentCreateNameIssue } from "../hooks/use-agent-create-view-state";
import { agentCreateAppearance, type AgentCreateDialogDraft } from "../lib/agent-create-draft";
import { AGENT_INSTRUCTION_TEMPLATES } from "../lib/agent-instruction-templates";
import { AgentAvatarChooser } from "./agent-avatar-chooser";
import { AgentFillWithAi } from "./agent-fill-with-ai";
import { AgentMark } from "./agent-mark";

const INLINE_FIELD =
  "-ml-2.5 h-auto w-[calc(100%+0.625rem)] border-transparent bg-transparent px-2.5 shadow-none hover:bg-hover focus-visible:border-transparent focus-visible:bg-hover focus-visible:shadow-none";

function NameMessage({
  issue,
  onUse,
}: {
  issue: AgentCreateNameIssue;
  onUse: (name: string) => void;
}) {
  if (!issue) return null;
  if (issue.kind === "no-letters") {
    return (
      <FieldError data-testid="agent-create-name-error">
        Add at least one letter or number to the name.
      </FieldError>
    );
  }
  return (
    <div className="flex items-center gap-2">
      <FieldError data-testid="agent-create-name-error">
        You already have an agent with this name.
      </FieldError>
      <Button onClick={() => onUse(issue.suggestion)} size="xs" type="button" variant="secondary">
        Use {issue.suggestion}
      </Button>
    </div>
  );
}

export function AgentCreateIdentity({
  draft,
  issue,
  onChange,
}: {
  draft: AgentCreateDialogDraft;
  issue: AgentCreateNameIssue;
  onChange: (patch: Partial<AgentCreateDialogDraft>) => void;
}) {
  return (
    <div className="flex items-center gap-5">
      <AgentAvatarChooser
        appearance={agentCreateAppearance(draft)}
        name={draft.displayName}
        onChange={appearance => onChange({ appearance })}
      />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <Input
          aria-invalid={issue ? true : undefined}
          aria-label="Name"
          autoFocus
          className={cn(
            INLINE_FIELD,
            "py-1.5 text-ui-title-lg leading-normal font-medium placeholder:text-subtle"
          )}
          data-testid="agent-create-name"
          maxLength={80}
          onChange={event => onChange({ displayName: event.target.value })}
          placeholder="Name your agent"
          value={draft.displayName}
        />
        <Input
          aria-label="What it does"
          className={cn(INLINE_FIELD, "py-1 text-body text-fg-2 placeholder:text-subtle")}
          data-testid="agent-create-description"
          maxLength={280}
          onChange={event => onChange({ description: event.target.value })}
          placeholder="What does it do?"
          value={draft.description}
        />
        <NameMessage issue={issue} onUse={displayName => onChange({ displayName, name: "" })} />
      </div>
    </div>
  );
}

export function AgentCreateIdentityRead({ draft }: { draft: AgentCreateDialogDraft }) {
  return (
    <div className="flex items-center gap-5">
      <div className="grid size-28 shrink-0 place-items-center">
        <AgentMark
          appearance={agentCreateAppearance(draft)}
          label={draft.displayName}
          quietIdle
          size={92}
        />
      </div>
      <div className="flex min-w-0 flex-col gap-1">
        <h2 className="truncate text-ui-title-lg leading-normal font-medium text-fg">
          {draft.displayName}
        </h2>
        <p className="text-body text-fg-2">{draft.description}</p>
      </div>
    </div>
  );
}

/** Words someone wrote stay: a starting point joins them instead of replacing them. */
function withTemplate(current: string, body: string) {
  const own =
    current.trim() !== "" && !AGENT_INSTRUCTION_TEMPLATES.some(item => item.body === current);
  return own ? `${current}\n\n${body}` : body;
}

export function AgentCreateInstructions({
  draft,
  error,
  onChange,
  providers,
  models,
}: {
  draft: AgentCreateDialogDraft;
  error?: string;
  onChange: (next: string) => void;
  providers: RuntimeProviderOption[];
  models: RuntimeModelOption[];
}) {
  const value = draft.prompt;
  return (
    <Field className="flex-[1_0_auto]" data-invalid={Boolean(error)}>
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <FieldLabel htmlFor="agent-create-prompt">Instructions</FieldLabel>
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="mr-1 text-eyebrow text-subtle">Start from</span>
          {AGENT_INSTRUCTION_TEMPLATES.map(template => (
            <Button
              aria-pressed={value === template.body}
              key={template.id}
              onClick={() => onChange(withTemplate(value, template.body))}
              size="xs"
              type="button"
              variant="secondary"
            >
              {value === template.body ? <Check aria-hidden="true" /> : null}
              {template.label}
            </Button>
          ))}
          <AgentFillWithAi
            models={models}
            onChange={onChange}
            providers={providers}
            runtime={{
              provider: draft.provider,
              model: draft.model,
              reasoning_effort: draft.reasoningEffort,
            }}
            value={value}
          />
        </div>
      </div>
      <MarkdownEditor
        className="min-h-64 flex-1 [&>div:last-child]:max-h-none [&>div:last-child]:flex-1"
        contentClassName="min-h-52"
        data-testid="agent-create-prompt"
        id="agent-create-prompt"
        onValueChange={onChange}
        placeholder="Write it the way you'd brief a new teammate."
        value={value}
      />
      <FieldError>{error}</FieldError>
    </Field>
  );
}

export function AgentCreateInstructionsRead({ value }: { value: string }) {
  return (
    <section className="flex flex-col gap-2">
      <h3 className="text-form-label text-muted-foreground">Instructions</h3>
      <div className="rounded-md bg-sunken p-4 text-small-body text-fg">
        <StreamMarkdown>{value}</StreamMarkdown>
      </div>
    </section>
  );
}
