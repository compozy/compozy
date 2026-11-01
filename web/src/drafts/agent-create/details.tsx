/** The left column: who the agent is, what it follows, and the one quiet Advanced fold. */
import { Check } from "lucide-react";
import { useState } from "react";

import {
  Button,
  Disclosure,
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  Input,
  MarkdownEditor,
  StreamMarkdown,
  ToggleGroup,
  ToggleGroupItem,
  cn,
} from "@compozy/ui";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import type { AgentState } from "../persistent-agents-shared/agent-state";

import { AvatarChooser } from "./avatar-chooser";
import { TEMPLATES, shortNameFor, type Draft, type NameIssue } from "./model";

const INLINE_FIELD =
  "-ml-2.5 h-auto w-[calc(100%+0.625rem)] border-transparent bg-transparent px-2.5 shadow-none hover:bg-hover focus-visible:border-transparent focus-visible:bg-hover focus-visible:shadow-none";

function NameMessage({ issue, onUse }: { issue: NameIssue; onUse: (name: string) => void }) {
  if (!issue) return null;
  if (issue.kind === "no-letters") {
    return <FieldError>Add at least one letter or number to the name.</FieldError>;
  }
  return (
    <div className="flex items-center gap-2">
      <FieldError>You already have an agent with this name.</FieldError>
      <Button onClick={() => onUse(issue.suggestion)} size="xs" type="button" variant="secondary">
        Use {issue.suggestion}
      </Button>
    </div>
  );
}

export interface IdentityProps {
  draft: Draft;
  issue: NameIssue;
  onChange: (patch: Partial<Draft>) => void;
  avatarOpen?: "generated" | "symbol";
}

export function Identity({ draft, issue, onChange, avatarOpen }: IdentityProps) {
  return (
    <div className="flex items-center gap-5">
      <AvatarChooser
        appearance={draft.appearance}
        defaultOpen={avatarOpen}
        name={draft.name}
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
          onChange={event => onChange({ name: event.target.value })}
          placeholder="Name your agent"
          value={draft.name}
        />
        <Input
          aria-label="What it does"
          className={cn(INLINE_FIELD, "py-1 text-body text-fg-2 placeholder:text-subtle")}
          onChange={event => onChange({ description: event.target.value })}
          placeholder="What does it do?"
          value={draft.description}
        />
        <NameMessage issue={issue} onUse={name => onChange({ name })} />
      </div>
    </div>
  );
}

export function IdentityRead({ draft, state }: { draft: Draft; state?: AgentState }) {
  return (
    <div className="flex items-center gap-5">
      <div className="grid size-28 shrink-0 place-items-center">
        <AgentAvatar
          appearance={draft.appearance}
          label={draft.name}
          quietIdle
          size={92}
          state={state}
        />
      </div>
      <div className="flex min-w-0 flex-col gap-1">
        <h2 className="truncate text-ui-title-lg leading-normal font-medium text-fg">
          {draft.name}
        </h2>
        <p className="text-body text-fg-2">{draft.description}</p>
      </div>
    </div>
  );
}

/** Words someone wrote stay: a starting point joins them instead of replacing them. */
function withTemplate(current: string, body: string) {
  const own = current.trim() !== "" && !TEMPLATES.some(item => item.body === current);
  return own ? `${current}\n\n${body}` : body;
}

export function Instructions({
  value,
  onChange,
}: {
  value: string;
  onChange: (next: string) => void;
}) {
  return (
    <Field className="min-h-0 flex-1">
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <FieldLabel htmlFor="agent-instructions">Instructions</FieldLabel>
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="mr-1 text-eyebrow text-subtle">Start from</span>
          {TEMPLATES.map(template => (
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
        </div>
      </div>
      <MarkdownEditor
        className="min-h-64 flex-1 [&>div:last-child]:max-h-none [&>div:last-child]:flex-1"
        contentClassName="min-h-52"
        id="agent-instructions"
        onValueChange={onChange}
        placeholder="Write it the way you'd brief a new teammate."
        value={value}
      />
    </Field>
  );
}

export function InstructionsRead({ value }: { value: string }) {
  return (
    <section className="flex flex-col gap-2">
      <h3 className="text-form-label text-muted">Instructions</h3>
      <div className="rounded-md bg-sunken p-4 text-small-body text-fg">
        <StreamMarkdown>{value}</StreamMarkdown>
      </div>
    </section>
  );
}

const TOOLS = ["Files", "Terminal", "Sessions", "Browser"];

export function Advanced({
  draft,
  defaultOpen,
  onChange,
}: {
  draft: Draft;
  defaultOpen?: boolean;
  onChange: (patch: Partial<Draft>) => void;
}) {
  const [tools, setTools] = useState<string[]>(TOOLS);
  return (
    <Disclosure defaultOpen={defaultOpen} label="Advanced" size="md">
      <div className="grid grid-cols-2 gap-6 pt-3">
        <Field>
          <FieldLabel htmlFor="agent-short-name">Short name</FieldLabel>
          <Input
            className="font-mono"
            id="agent-short-name"
            onChange={event => onChange({ shortName: event.target.value })}
            placeholder={shortNameFor(draft.name) || "reviewer"}
            value={draft.shortName ?? ""}
          />
          <FieldDescription>Used to mention it in a message.</FieldDescription>
        </Field>
        <Field>
          <FieldLabel>What it can use</FieldLabel>
          <ToggleGroup
            aria-label="What it can use"
            multiple
            onValueChange={setTools}
            size="sm"
            value={tools}
          >
            {TOOLS.map(tool => (
              <ToggleGroupItem key={tool} value={tool}>
                {tool}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        </Field>
      </div>
    </Disclosure>
  );
}
