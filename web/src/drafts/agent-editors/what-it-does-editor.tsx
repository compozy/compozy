/** S6: what the agent does, in its own words first; tone and hard limits are optional additions. */
import { Ban, Info, Plus, X } from "lucide-react";
import { useState } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  Eyebrow,
  Input,
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
  MarkdownEditor,
  Pill,
  PillGroup,
  toast,
} from "@compozy/ui";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import { AgentAppPage } from "../persistent-agents-shared/agent-page";
import type { DraftAgent } from "../persistent-agents-shared/fixtures";
import { EditorCard } from "../persistent-agents-shared/rich-controls";
import { ConflictCard } from "./conflict-card";
import { EditorHead } from "./editor-head";
import { RawFile } from "./raw-file";
import { UnsavedDock } from "./unsaved-dock";

export type WhatItDoesVariant =
  | "filled"
  | "empty"
  | "unsaved"
  | "conflict"
  | "advanced"
  | "advanced-only"
  | "invalid";

type Tone = "direct" | "friendly" | "formal" | "custom";

interface Draft {
  body: string;
  tone: Tone;
  customTone: string;
  never: readonly string[];
  raw: string | null;
}

const SAVED: Draft = {
  body: [
    "Keep this project's issues moving.",
    "",
    "Every morning, look at what's open and work out what's stuck. Name who can unblock it, and suggest the next step.",
    "",
    "- Hand building work to **Code Engineer**",
    "- Hand reviews to **Reviewer**",
    "- Stay quiet when nothing changed",
  ].join("\n"),
  tone: "direct",
  customTone: "",
  never: ["Merge anything on its own", "Close an issue without asking"],
  raw: null,
};

const BLANK: Draft = { body: "", tone: "direct", customTone: "", never: [], raw: null };

const THEIR_LINE = "Flag anything that's been waiting more than 3 days.";

const EXAMPLES = [
  {
    label: "Keep issues moving",
    body: "Look at open issues every morning. Work out what's stuck, name who can unblock it, and suggest the next step.",
  },
  {
    label: "Review pull requests",
    body: "Read each new pull request. Point out what could break, and say clearly when it's good to go.",
  },
  {
    label: "Write a weekly update",
    body: "Every Friday, sum up what shipped this week and what's next, in a few short lines.",
  },
];

const TONES: readonly { value: Tone; label: string; sample: string }[] = [
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

const ADVANCED_ONLY_FILE = `---
version: 1
tone: direct
---

# Release checklist

| Step | Who |
| --- | --- |
| Tag | Release Captain |
| Notes | Triage Lead |
`;

function composeFile(draft: Draft) {
  const never = draft.never.map(item => `  - ${item}`).join("\n");
  const tone = draft.tone === "custom" ? draft.customTone : draft.tone;
  return [
    "---",
    "version: 1",
    `tone: ${tone}`,
    ...(never ? ["constraints:", never] : []),
    "---",
    "",
    draft.body,
  ].join("\n");
}

function fileError(raw: string | null) {
  if (raw === null) return undefined;
  const index = raw.split("\n").findIndex(line => line.includes("[") && !line.includes("]"));
  if (index < 0) return undefined;
  return `Line ${index + 1} has a [ that never closes. Fix it to save.`;
}

function countChanges(draft: Draft, saved: Draft) {
  const toneChanged = draft.tone !== saved.tone || draft.customTone !== saved.customTone;
  const neverChanged = draft.never.join() !== saved.never.join();
  return [draft.body !== saved.body, toneChanged, neverChanged, draft.raw !== null].filter(Boolean)
    .length;
}

function startingDraft(variant: WhatItDoesVariant): Draft {
  if (variant === "empty") return BLANK;
  if (variant === "unsaved" || variant === "conflict") {
    return {
      ...SAVED,
      body: `${SAVED.body}\n- Say who's on call when something breaks`,
      tone: "friendly",
    };
  }
  if (variant === "invalid") {
    return { ...SAVED, raw: composeFile(SAVED).replace("tone: direct", "tone: [direct") };
  }
  return SAVED;
}

function SoundCard({
  agent,
  draft,
  onChange,
}: {
  agent: DraftAgent;
  draft: Draft;
  onChange: (patch: Partial<Draft>) => void;
}) {
  const tone = TONES.find(item => item.value === draft.tone)!;
  return (
    <EditorCard className="gap-4">
      <h2 className="text-item-title font-medium text-fg">How it should sound</h2>
      <PillGroup
        aria-label="How it should sound"
        items={TONES.map(({ value, label }) => ({ value, label }))}
        onChange={value => onChange({ tone: value })}
        value={draft.tone}
      />
      {draft.tone === "custom" ? (
        <Input
          aria-label="Describe how it should sound"
          autoFocus
          onChange={event => onChange({ customTone: event.target.value })}
          placeholder="Calm, a little dry, no exclamation marks"
          value={draft.customTone}
        />
      ) : (
        <div className="flex items-end gap-2">
          <AgentAvatar
            appearance={agent.appearance}
            label={agent.label}
            quietIdle
            size={28}
            surfaceClassName="ring-card"
          />
          <p className="min-w-0 rounded-lg rounded-bl-xxs bg-sunken px-3 py-2 text-small-body leading-relaxed text-fg-2">
            {tone.sample}
          </p>
        </div>
      )}
    </EditorCard>
  );
}

function NeverCard({
  items,
  onChange,
}: {
  items: readonly string[];
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
              <Ban aria-hidden="true" className="size-3.5 shrink-0 text-muted" />
              <span className="min-w-0 flex-1 text-small-body text-fg">{item}</span>
              <Button
                aria-label={`Remove ${item}`}
                onClick={() => onChange(items.filter(other => other !== item))}
                size="icon-xs"
                type="button"
                variant="quiet"
              >
                <X aria-hidden="true" />
              </Button>
            </li>
          ))}
        </ul>
      ) : null}
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
            <InputGroupButton aria-label="Add" disabled={!text.trim()} size="icon-xs" type="submit">
              <Plus aria-hidden="true" />
            </InputGroupButton>
          </InputGroupAddon>
        </InputGroup>
      </form>
      {ideas.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {ideas.map(idea => (
            <Pill key={idea} render={<button onClick={() => add(idea)} type="button" />} size="sm">
              <Plus aria-hidden="true" />
              {idea}
            </Pill>
          ))}
        </div>
      ) : null}
    </EditorCard>
  );
}

export function WhatItDoesEditor({
  agent,
  variant,
}: {
  agent: DraftAgent;
  variant: WhatItDoesVariant;
}) {
  const [saved, setSaved] = useState(variant === "empty" ? BLANK : SAVED);
  const [draft, setDraft] = useState(() => startingDraft(variant));
  const [conflict, setConflict] = useState(variant === "conflict");
  const advancedOnly = variant === "advanced-only";
  const patch = (next: Partial<Draft>) => setDraft(current => ({ ...current, ...next }));
  const error = fileError(draft.raw);
  const changes = countChanges(draft, saved);
  const theirs = `${SAVED.body}\n- ${THEIR_LINE}`;

  return (
    <AgentAppPage agent={agent} hubState="what" leaf="What it does" width="max-w-5xl">
      <div className="flex flex-col gap-6 py-2">
        <EditorHead agent={agent} subtitle={agent.description} title="What it does" />
        {conflict ? (
          <ConflictCard
            agentLabel={agent.label}
            mine={<p>Adds "Say who's on call when something breaks" and sounds friendly.</p>}
            onKeepMine={() => {
              setConflict(false);
              toast("Kept your version");
            }}
            onUseTheirs={() => {
              const next = { ...SAVED, body: theirs };
              setSaved(next);
              setDraft(next);
              setConflict(false);
              toast(`Using ${agent.label}'s version`);
            }}
            theirs={<p>Adds "{THEIR_LINE}"</p>}
            title={`${agent.label} changed this while you were editing`}
          />
        ) : null}
        {advancedOnly ? (
          <>
            <Alert variant="info">
              <Info aria-hidden="true" className="size-4" />
              <AlertDescription>
                This file has a table the editor can't show, so it opens here. Nothing gets lost.
              </AlertDescription>
            </Alert>
            <RawFile
              defaultOpen
              onChange={value => patch({ raw: value })}
              value={draft.raw ?? ADVANCED_ONLY_FILE}
            />
          </>
        ) : (
          <div className="grid grid-cols-[minmax(0,1fr)_21rem] items-start gap-5">
            <div className="flex min-w-0 flex-col gap-4">
              <MarkdownEditor
                aria-label="What it does"
                autoFocus={variant === "empty"}
                className="bg-card [&>.overflow-y-auto]:max-h-none"
                contentClassName="min-h-96 px-4 py-3 text-body leading-relaxed"
                disabled={draft.raw !== null}
                onValueChange={value => patch({ body: value })}
                placeholder={`Tell ${agent.label} what you want it to do, the way you'd tell a teammate.`}
                value={draft.body}
              />
              {draft.body === "" ? (
                <div className="flex flex-wrap items-center gap-2">
                  <Eyebrow>Start from an example</Eyebrow>
                  {EXAMPLES.map(example => (
                    <Button
                      key={example.label}
                      onClick={() => patch({ body: example.body })}
                      size="sm"
                      type="button"
                      variant="secondary"
                    >
                      {example.label}
                    </Button>
                  ))}
                </div>
              ) : null}
              <RawFile
                defaultOpen={variant === "advanced" || variant === "invalid"}
                error={error}
                onChange={value => patch({ raw: value })}
                value={draft.raw ?? composeFile(draft)}
              />
            </div>
            <div className="flex min-w-0 flex-col gap-4">
              <SoundCard agent={agent} draft={draft} onChange={patch} />
              <NeverCard items={draft.never} onChange={never => patch({ never })} />
            </div>
          </div>
        )}
        <UnsavedDock
          blocked={error ? "Fix the file first" : undefined}
          count={changes}
          onDiscard={() => setDraft(saved)}
          onSave={() => {
            const next = { ...draft, raw: null };
            setSaved(next);
            setDraft(next);
            toast("Saved. It takes effect from the next session.");
          }}
        />
      </div>
    </AgentAppPage>
  );
}
