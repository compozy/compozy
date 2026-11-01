/** S9: notes the person can read, find, add and remove, with how full memory is and history. */
import { Plus, SearchX } from "@compozy/ui/icons";
import type { AgentAvatarState } from "@compozy/ui";

import { Button, Empty, PillGroup, SearchInput, toast } from "@compozy/ui";

import type { MemoryEditorModel } from "../../hooks/use-memory-editor";
import { agentEditorMark } from "../../lib/agent-editor-mark";
import { describeChanges } from "../../lib/agent-frontmatter";
import { memoryBudgetLine, type MemoryFilter, type MemoryNote } from "../../lib/agent-memory-notes";
import type { AgentPayload } from "../../types";
import { AgentMark } from "../agent-mark";
import { ConflictCard } from "./conflict-card";
import { EditorHead } from "./editor-head";
import { FullnessCard, HistoryCard } from "./memory-side";
import { NoteBoard, NoteCard, NoteComposer, NotePiles, NoteSheet } from "./memory-note-card";
import { RawFile } from "./raw-file";
import { EditorCard } from "./rich-controls";
import { UnsavedDock } from "./unsaved-dock";

const FILTERS: readonly { value: MemoryFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "learned", label: "Learned" },
  { value: "yours", label: "Yours" },
];

const STARTERS = [
  "Keep updates short",
  "Ask before changing anything big",
  "Talk to me in Portuguese",
];

function noteLines(notes: readonly MemoryNote[]) {
  return notes.map(note => note.text).join("\n");
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

export interface AgentMemoryViewProps {
  scopeLabel?: (scope: string) => string;
  agent: AgentPayload;
  state: AgentAvatarState;
  model: MemoryEditorModel;
}

export function AgentMemoryView({ agent, state, model, scopeLabel }: AgentMemoryViewProps) {
  const { document, notes, saved, shown, adding, seed, search, filter, today } = model;
  const { percent, fullness, full, learned, budget, revisions } = model;
  const { conflict, raw, baseline, problem } = document;
  const label = agent.label || agent.name;
  const theirNotes = conflict ? conflict.form.notes : [];
  const theirChanges = describeChanges(noteLines(baseline.form.notes), noteLines(theirNotes));
  const myChanges = describeChanges(noteLines(saved), noteLines(notes));
  return (
    <div className="flex flex-col gap-6 py-2">
      <EditorHead
        agent={agent}
        state={state}
        subtitle="What it remembers wherever it works"
        title="Memory"
      >
        <Button disabled={full || adding} onClick={() => model.startAdding()} type="button">
          <Plus aria-hidden="true" />
          Add a note
        </Button>
      </EditorHead>
      {conflict ? (
        <ConflictCard
          agentLabel={label}
          mine={<Lines lines={raw === null ? myChanges : ["Your edits to the file"]} />}
          onKeepMine={() => void document.save()}
          onUseTheirs={() => {
            document.chooseTheirs();
            toast.message(`Using ${label}'s version`);
          }}
          theirs={<Lines lines={theirChanges} />}
          title={
            theirNotes.length > baseline.form.notes.length
              ? `${label} added a note while you were editing`
              : `${label} changed its notes while you were editing`
          }
        />
      ) : null}
      <div className="grid grid-cols-[minmax(0,1fr)_20rem] items-start gap-5">
        <div className="flex min-w-0 flex-col gap-4">
          {adding ? (
            <NoteBoard>
              <NoteSheet>
                <NoteComposer
                  action="Add note"
                  initial={seed}
                  onCancel={model.cancelAdding}
                  onDone={model.add}
                />
              </NoteSheet>
            </NoteBoard>
          ) : null}
          {notes.length === 0 ? (
            adding ? null : (
              <EditorCard className="py-12">
                <Empty
                  description="Notes show up here as it works. You can also give it one now."
                  fill={false}
                  icon={<AgentMark {...agentEditorMark(agent)} quietIdle size={40} />}
                  nextSteps={STARTERS.map(starter => (
                    <Button
                      key={starter}
                      onClick={() => model.startAdding(starter)}
                      size="sm"
                      type="button"
                      variant="secondary"
                    >
                      {starter}
                    </Button>
                  ))}
                  title={`${label} hasn't learned anything yet`}
                />
              </EditorCard>
            )
          ) : (
            <>
              <div className="flex items-center gap-3">
                <SearchInput
                  aria-label="Search notes"
                  containerClassName="min-w-0 flex-1"
                  onChange={model.setSearch}
                  placeholder="Search notes"
                  value={search}
                />
                <PillGroup items={FILTERS} onChange={model.setFilter} value={filter} />
              </div>
              {shown.length > 0 ? (
                <NoteBoard>
                  <NotePiles
                    notes={shown}
                    render={note => (
                      <NoteCard
                        agent={agent}
                        key={note.id}
                        note={note}
                        onEdit={text => model.update(note.id, text)}
                        onRemove={() => model.remove(note.id)}
                        pending={
                          !saved.some(other => other.id === note.id && other.text === note.text)
                        }
                        scopeLabel={scopeLabel}
                        today={today}
                      />
                    )}
                  />
                </NoteBoard>
              ) : (
                <Empty
                  description="Try another word, or show all notes."
                  fill={false}
                  icon={SearchX}
                  size="compact"
                  title="No notes match"
                />
              )}
            </>
          )}
          <RawFile
            defaultOpen={Boolean(problem)}
            error={problem}
            note={memoryBudgetLine(budget)}
            onChange={document.setRaw}
            value={document.source}
          />
        </div>
        <div className="sticky top-4 flex min-w-0 flex-col gap-4">
          <FullnessCard
            fullness={fullness}
            learned={learned}
            onShowYours={() => model.setFilter("yours")}
            percent={percent}
            yours={notes.length - learned}
          />
          {notes.length > 0 && revisions.length > 0 ? (
            <HistoryCard
              disabled={document.changes > 0 || model.restoring}
              onRestore={model.restore}
              revisions={revisions}
            />
          ) : null}
        </div>
      </div>
      <UnsavedDock
        blocked={problem ? "Fix the file first" : undefined}
        count={document.changes}
        onDiscard={document.discard}
        onSave={() => void document.save()}
        saving={document.saving}
      />
    </div>
  );
}
