/** S6: what the agent does, in its own words first; tone and hard limits are optional additions. */
import type { AgentAvatarState } from "@compozy/ui";

import { Button, Eyebrow, MarkdownEditor, toast } from "@compozy/ui";

import type { EditorDocument } from "../../hooks/use-editor-document";
import { describeChanges } from "../../lib/agent-frontmatter";
import type { SoulForm } from "../../lib/agent-soul-form";
import type { AgentPayload } from "../../types";
import { ConflictCard } from "./conflict-card";
import { EditorHead } from "./editor-head";
import { FileBlockNotice, ReadOnlyNotice } from "./editor-states";
import { RawFile } from "./raw-file";
import { UnsavedDock } from "./unsaved-dock";
import { NeverCard, SoundCard } from "./what-it-does-cards";

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

function toneLine(from: SoulForm, to: SoulForm) {
  if (from.tone === to.tone && from.customTone === to.customTone) return [];
  return [`Sounds ${to.tone === "custom" ? to.customTone : (to.tone ?? "its own way")}`];
}

function changes(from: SoulForm, to: SoulForm) {
  return [
    ...describeChanges(from.body, to.body),
    ...toneLine(from, to),
    ...describeChanges(from.never.join("\n"), to.never.join("\n"), 1),
  ];
}

function Lines({ lines }: { lines: readonly string[] }) {
  return (
    <>
      {lines.map(line => (
        <p key={line}>{line}</p>
      ))}
    </>
  );
}

export interface AgentWhatItDoesViewProps {
  agent: AgentPayload;
  state: AgentAvatarState;
  readOnly: boolean;
  document: EditorDocument<SoulForm>;
}

function WhatItDoesConflict({
  label,
  document,
}: {
  label: string;
  document: EditorDocument<SoulForm>;
}) {
  const { form, raw, baseline, conflict } = document;
  if (!conflict) return null;
  return (
    <ConflictCard
      agentLabel={label}
      mine={
        <Lines lines={raw === null ? changes(baseline.form, form) : ["Your edits to the file"]} />
      }
      onKeepMine={() => void document.save()}
      onUseTheirs={() => {
        document.chooseTheirs();
        toast.message(`Using ${label}'s version`);
      }}
      theirs={
        <Lines
          lines={
            conflict.block
              ? ["Changes the file in a way this page can't show"]
              : changes(baseline.form, conflict.form)
          }
        />
      }
      title={`${label} changed this while you were editing`}
    />
  );
}

function ExampleStarters({ onPick }: { onPick: (body: string) => void }) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Eyebrow>Start from an example</Eyebrow>
      {EXAMPLES.map(example => (
        <Button
          key={example.label}
          onClick={() => onPick(example.body)}
          size="sm"
          type="button"
          variant="secondary"
        >
          {example.label}
        </Button>
      ))}
    </div>
  );
}

function WhatItDoesForm({
  agent,
  label,
  readOnly,
  document,
}: Pick<AgentWhatItDoesViewProps, "agent" | "readOnly" | "document"> & { label: string }) {
  const { form, raw, baseline, problem } = document;
  const patch = (next: Partial<SoulForm>) => document.setForm({ ...form, ...next });
  const locked = readOnly || raw !== null;
  return (
    <div className="grid grid-cols-[minmax(0,1fr)_21rem] items-start gap-5">
      <div className="flex min-w-0 flex-col gap-4">
        <MarkdownEditor
          aria-label="What it does"
          autoFocus={baseline.missing === true && !readOnly}
          className="bg-card"
          contentClassName="min-h-96 px-4 py-3 text-body leading-relaxed"
          disabled={locked}
          height="tall"
          onValueChange={body => patch({ body })}
          placeholder={`Tell ${label} what you want it to do, the way you'd tell a teammate.`}
          value={form.body}
        />
        {form.body === "" && !readOnly ? (
          <ExampleStarters onPick={body => patch({ body })} />
        ) : null}
        <RawFile
          defaultOpen={Boolean(problem)}
          error={problem}
          note={form.kept.tags.length > 0 ? `Tags: ${form.kept.tags.join(", ")}` : undefined}
          onChange={document.setRaw}
          readOnly={readOnly}
          value={document.source}
        />
      </div>
      <div className="flex min-w-0 flex-col gap-4">
        <SoundCard agent={agent} disabled={locked} form={form} onChange={patch} />
        <NeverCard disabled={locked} items={form.never} onChange={never => patch({ never })} />
      </div>
    </div>
  );
}

export function AgentWhatItDoesView({
  agent,
  state,
  readOnly,
  document,
}: AgentWhatItDoesViewProps) {
  const { block, problem } = document;
  const label = agent.label || agent.name;
  return (
    <div className="flex flex-col gap-6 py-2">
      <EditorHead agent={agent} state={state} subtitle={agent.description} title="What it does" />
      {readOnly ? <ReadOnlyNotice agent={agent} /> : null}
      <WhatItDoesConflict document={document} label={label} />
      {block ? (
        <>
          <FileBlockNotice block={block} />
          <RawFile
            defaultOpen
            error={problem}
            onChange={document.setRaw}
            readOnly={readOnly}
            value={document.source}
          />
        </>
      ) : (
        <WhatItDoesForm agent={agent} document={document} label={label} readOnly={readOnly} />
      )}
      {readOnly ? null : (
        <UnsavedDock
          blocked={problem ? "Fix the file first" : undefined}
          count={document.changes}
          onDiscard={document.discard}
          onSave={() => void document.save()}
          saving={document.saving}
        />
      )}
    </div>
  );
}
