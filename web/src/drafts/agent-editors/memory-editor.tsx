/** S9: notes the person can read, find, add and remove, with how full memory is and history. */
import { Plus, SearchX } from "lucide-react";

import { Button, Empty, PillGroup, SearchInput, toast } from "@compozy/ui";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import { AgentAppPage } from "../stays-on-agents-shared/agent-page";
import type { DraftAgent } from "../stays-on-agents-shared/fixtures";
import { EditorCard } from "../stays-on-agents-shared/rich-controls";
import { ConflictCard } from "./conflict-card";
import { EditorHead } from "./editor-head";
import { useMemoryEditor, type Filter, type MemoryVariant } from "./hooks/use-memory-editor";
import { FILE_LIMITS, THEIR_NOTE, type Note } from "./memory-fixtures";
import { NoteCard, NoteComposer } from "./memory-note-card";
import { FullnessCard, HistoryCard, PutAwayCard } from "./memory-side";
import { RawFile } from "./raw-file";
import { UnsavedDock } from "./unsaved-dock";

export type { MemoryVariant };

const FILTERS: readonly { value: Filter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "learned", label: "Learned" },
  { value: "yours", label: "Yours" },
];

const STARTERS = [
  "Keep updates short",
  "Ask before changing anything big",
  "Talk to me in Portuguese",
];

function composeFile(notes: readonly Note[]) {
  return notes
    .map(note =>
      note.learnedIn
        ? `- ${note.date} · ${note.learnedIn} · ${note.text}${note.until ? ` · until ${note.until}` : ""}`
        : note.text
    )
    .join("\n");
}

export function MemoryEditor({ agent, variant }: { agent: DraftAgent; variant: MemoryVariant }) {
  const editor = useMemoryEditor(variant);
  const { notes, saved, raw, adding, seed, search, filter, conflict, restored } = editor;
  const { percent, fullness, full, learned, shown, changes } = editor;

  return (
    <AgentAppPage agent={agent} hubState="memory" leaf="Memory" width="max-w-5xl">
      <div className="flex flex-col gap-6 py-2">
        <EditorHead agent={agent} subtitle="What it remembers wherever it works" title="Memory">
          <Button disabled={full || adding} onClick={() => editor.startAdding()} type="button">
            <Plus aria-hidden="true" />
            Add a note
          </Button>
        </EditorHead>
        {conflict ? (
          <ConflictCard
            agentLabel={agent.label}
            mine={
              <p>Adds a note about Ana being out and removes the one about the checkout test.</p>
            }
            onKeepMine={editor.keepMine}
            onUseTheirs={() => {
              editor.useTheirs();
              toast(`Using ${agent.label}'s version`);
            }}
            theirs={<p>Adds "{THEIR_NOTE.text}"</p>}
            title={`${agent.label} added a note while you were editing`}
          />
        ) : null}
        <div className="grid grid-cols-[minmax(0,1fr)_20rem] items-start gap-5">
          <div className="flex min-w-0 flex-col gap-4">
            {adding ? (
              <EditorCard className="p-4">
                <NoteComposer
                  action="Add note"
                  initial={seed}
                  onCancel={editor.cancelAdding}
                  onDone={editor.add}
                />
              </EditorCard>
            ) : null}
            {notes.length === 0 ? (
              adding ? null : (
                <EditorCard className="py-12">
                  <Empty
                    description="Notes show up here as it works. You can also give it one now."
                    fill={false}
                    icon={
                      <AgentAvatar
                        appearance={agent.appearance}
                        label={agent.label}
                        quietIdle
                        size={40}
                      />
                    }
                    nextSteps={STARTERS.map(starter => (
                      <Button
                        key={starter}
                        onClick={() => editor.startAdding(starter)}
                        size="sm"
                        type="button"
                        variant="secondary"
                      >
                        {starter}
                      </Button>
                    ))}
                    title={`${agent.label} hasn't learned anything yet`}
                  />
                </EditorCard>
              )
            ) : (
              <>
                <div className="flex items-center gap-3">
                  <SearchInput
                    aria-label="Search notes"
                    containerClassName="min-w-0 flex-1"
                    onChange={editor.setSearch}
                    placeholder="Search notes"
                    value={search}
                  />
                  <PillGroup items={FILTERS} onChange={editor.setFilter} value={filter} />
                </div>
                {shown.length > 0 ? (
                  <ul className="columns-2 gap-3">
                    {shown.map(note => (
                      <NoteCard
                        key={note.id}
                        note={note}
                        onEdit={text => editor.update(note.id, text)}
                        onRemove={() => editor.remove(note.id)}
                        pending={
                          !saved.some(other => other.id === note.id && other.text === note.text)
                        }
                      />
                    ))}
                  </ul>
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
              defaultOpen={variant === "advanced"}
              note={`${Math.round((FILE_LIMITS.lines * percent) / 100)} of ${FILE_LIMITS.lines} lines, ${Math.round((FILE_LIMITS.size * percent) / 100)} of ${FILE_LIMITS.size} KB used`}
              onChange={editor.setRaw}
              value={raw ?? composeFile(notes)}
            />
          </div>
          <div className="sticky top-4 flex min-w-0 flex-col gap-4">
            <FullnessCard
              fullness={fullness}
              learned={learned}
              onShowYours={() => editor.setFilter("yours")}
              percent={percent}
              yours={notes.length - learned}
            />
            {variant === "almost-full" ? <PutAwayCard count={37} /> : null}
            {notes.length > 0 ? (
              <HistoryCard onRestore={editor.restore} restored={restored} />
            ) : null}
          </div>
        </div>
        <UnsavedDock count={changes} onDiscard={editor.discard} onSave={editor.save} />
      </div>
    </AgentAppPage>
  );
}
