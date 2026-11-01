/** S6's side column: how the agent sounds, and what it must never do. */
import { Ban, Plus, X } from "@compozy/ui/icons";
import { useState } from "react";

import {
  Button,
  Input,
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
  Pill,
  PillGroup,
} from "@compozy/ui";

import { agentEditorMark } from "../../lib/agent-editor-mark";
import type { SoulForm, SoulTone } from "../../lib/agent-soul-form";
import type { AgentPayload } from "../../types";
import { AgentMark } from "../agent-mark";
import { EditorCard } from "./rich-controls";

const TONES: readonly { value: SoulTone; label: string; sample: string }[] = [
  {
    value: "direct",
    label: "Direct",
    sample: "Token migration is stuck. Ana can unblock it. Want me to ask her?",
  },
  {
    value: "friendly",
    label: "Friendly",
    sample: "Heads up, the token migration got stuck. Ana could help. Should I ask her for you?",
  },
  {
    value: "formal",
    label: "Formal",
    sample:
      "The token migration is currently blocked. Ana is able to resolve this. Shall I contact her?",
  },
  { value: "custom", label: "Your own", sample: "" },
];

const NEVER_IDEAS = [
  "Merge anything on its own",
  "Delete files",
  "Message people outside the team",
  "Spend money",
];

export function SoundCard({
  agent,
  form,
  disabled,
  onChange,
}: {
  agent: AgentPayload;
  form: SoulForm;
  disabled: boolean;
  onChange: (patch: Partial<SoulForm>) => void;
}) {
  const tone = TONES.find(item => item.value === form.tone);
  return (
    <EditorCard className="gap-4">
      <h2 className="text-item-title font-medium text-fg">How it should sound</h2>
      <PillGroup<SoulTone | "">
        aria-label="How it should sound"
        items={TONES.map(({ value, label }) => ({ value, label, disabled }))}
        onChange={value => onChange({ tone: value === "" ? null : value })}
        value={form.tone ?? ""}
      />
      {form.tone === "custom" ? (
        <Input
          aria-label="Describe how it should sound"
          disabled={disabled}
          onChange={event => onChange({ customTone: event.target.value })}
          placeholder="Calm, a little dry, no exclamation marks"
          value={form.customTone}
        />
      ) : tone ? (
        <div className="flex items-end gap-2">
          <AgentMark {...agentEditorMark(agent)} quietIdle size={28} surfaceClassName="ring-card" />
          <p className="min-w-0 rounded-lg rounded-bl-xxs bg-sunken px-3 py-2 text-small-body leading-relaxed text-fg-2">
            {tone.sample}
          </p>
        </div>
      ) : null}
    </EditorCard>
  );
}

export function NeverCard({
  items,
  disabled,
  onChange,
}: {
  items: readonly string[];
  disabled: boolean;
  onChange: (next: readonly string[]) => void;
}) {
  const [text, setText] = useState("");
  const add = (value: string) => {
    const next = value.trim();
    if (next && !items.includes(next)) onChange([...items, next]);
    setText("");
  };
  const ideas = NEVER_IDEAS.filter(idea => !items.includes(idea)).slice(0, 3);
  return (
    <EditorCard className="gap-4">
      <h2 className="text-item-title font-medium text-fg">Things it must never do</h2>
      {items.length > 0 ? (
        <ul className="flex flex-col divide-y divide-line-soft">
          {items.map(item => (
            <li className="flex min-h-9 items-center gap-2 py-1" key={item}>
              <Ban aria-hidden="true" className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1 text-small-body text-fg">{item}</span>
              {disabled ? null : (
                <Button
                  aria-label={`Remove ${item}`}
                  onClick={() => onChange(items.filter(other => other !== item))}
                  size="icon-xs"
                  type="button"
                  variant="quiet"
                >
                  <X aria-hidden="true" />
                </Button>
              )}
            </li>
          ))}
        </ul>
      ) : null}
      {disabled ? null : (
        <>
          <form
            onSubmit={event => {
              event.preventDefault();
              add(text);
            }}
          >
            <InputGroup>
              <InputGroupInput
                aria-label="Add something it must never do"
                onChange={event => setText(event.target.value)}
                placeholder="Add another"
                value={text}
              />
              <InputGroupAddon align="inline-end">
                <InputGroupButton
                  aria-label="Add"
                  disabled={!text.trim()}
                  size="icon-xs"
                  type="submit"
                >
                  <Plus aria-hidden="true" />
                </InputGroupButton>
              </InputGroupAddon>
            </InputGroup>
          </form>
          {ideas.length > 0 ? (
            <div className="flex flex-wrap gap-1.5">
              {ideas.map(idea => (
                <Pill
                  key={idea}
                  render={<button onClick={() => add(idea)} type="button" />}
                  size="sm"
                >
                  <Plus aria-hidden="true" />
                  {idea}
                </Pill>
              ))}
            </div>
          ) : null}
        </>
      )}
    </EditorCard>
  );
}
