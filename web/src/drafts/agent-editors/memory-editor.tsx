/** S9: notes with their source scope, the budget, and history. */
import { Archive, Brain, CircleAlert, History, Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";

import {
  Alert,
  AlertActions,
  AlertDescription,
  AlertTitle,
  Button,
  Empty,
  Pill,
  PillGroup,
  Textarea,
  cn,
} from "@compozy/ui";

import { BudgetBar, EditorCard } from "../persistent-agents-shared/rich-controls";

interface Note {
  text: string;
  date?: string;
  scope?: string;
  until?: string;
  expired?: boolean;
}

const NOTES: readonly Note[] = [
  {
    date: "Nov 2",
    scope: "reloaddev",
    text: "Preview deploys need make preview.",
    until: "until Dec 1",
  },
  {
    date: "Oct 31",
    scope: "compozy",
    text: "Dispatch one executor per issue; steer instead of queueing urgent follow-ups.",
  },
  { date: "Oct 28", scope: "advos", text: "Hearing dates live in the calendar, not in tasks." },
  { text: "Prefer short status updates." },
];

const EXPIRED: readonly Note[] = [
  { ...NOTES[0]!, until: "expired Oct 1", expired: true },
  ...NOTES.slice(1),
];

const REVISIONS = [
  ["Today, 09:12", "Triage Lead added a note"],
  ["Oct 31, 18:40", "You edited 1 note"],
  ["Oct 28, 11:02", "Triage Lead added a note"],
];

export type MemoryState =
  | "notes"
  | "empty"
  | "near-budget"
  | "archive"
  | "conflict"
  | "expired"
  | "advanced"
  | "full";

function NoteRow({ note }: { note: Note }) {
  return (
    <li className="group flex items-start gap-3 px-4 py-3">
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <p className={cn("text-small-body", note.expired ? "text-subtle line-through" : "text-fg")}>
          {note.text}
        </p>
        <span className="flex items-center gap-2 text-eyebrow text-subtle">
          {note.scope ? <Pill size="xs">{note.scope}</Pill> : <Pill size="xs">You</Pill>}
          {note.date ? <span>{note.date}</span> : null}
          {note.until ? <span>· {note.until}</span> : null}
        </span>
      </div>
      <Button aria-label="Edit note" size="icon-xs" type="button" variant="quiet">
        <Pencil aria-hidden="true" />
      </Button>
      <Button aria-label="Remove note" size="icon-xs" type="button" variant="quiet">
        <Trash2 aria-hidden="true" />
      </Button>
    </li>
  );
}

const RAW = [
  "- 2026-11-02 · reloaddev · Preview deploys need make preview. · until 2026-12-01",
  "- 2026-10-31 · compozy · Dispatch one executor per issue.",
  "Prefer short status updates.",
].join("\n");

export function MemoryEditor({ state }: { state: MemoryState }) {
  const [mode, setMode] = useState(state === "advanced" ? "advanced" : "notes");
  const near = state === "near-budget" || state === "full";
  const empty = state === "empty";
  const notes = state === "expired" ? EXPIRED : NOTES;
  return (
    <div className="grid grid-cols-[minmax(0,1fr)_20rem] items-start gap-4 py-2">
      <div className="flex min-w-0 flex-col gap-4">
        {state === "conflict" ? (
          <Alert variant="warning">
            <CircleAlert aria-hidden="true" className="size-4" />
            <AlertTitle>Memory changed since you opened it</AlertTitle>
            <AlertActions>
              <Button size="xs" type="button" variant="secondary">
                Use theirs
              </Button>
              <Button size="xs" type="button" variant="secondary">
                Keep mine
              </Button>
            </AlertActions>
          </Alert>
        ) : null}
        {state === "full" ? (
          <Alert variant="danger">
            <AlertDescription>Memory is full of notes that can't be archived.</AlertDescription>
          </Alert>
        ) : null}
        <div className="flex items-center justify-between gap-2">
          <PillGroup
            items={[
              { value: "notes", label: "Notes" },
              { value: "advanced", label: "Advanced" },
            ]}
            onChange={setMode}
            value={mode}
          />
          <Button size="sm" type="button" variant="secondary">
            <Plus aria-hidden="true" />
            Add note
          </Button>
        </div>
        {empty ? (
          <Empty icon={Brain} title="No notes yet" />
        ) : mode === "notes" ? (
          <ul className="flex flex-col divide-y divide-line-soft overflow-hidden rounded-lg bg-card shadow-card">
            {notes.map(note => (
              <NoteRow key={note.text} note={note} />
            ))}
          </ul>
        ) : (
          <Textarea aria-label="MEMORY.md" className="min-h-96 font-mono" defaultValue={RAW} />
        )}
      </div>
      <div className="flex min-w-0 flex-col gap-4">
        <EditorCard className="gap-4">
          <BudgetBar label="Lines" max={200} unit="lines" value={empty ? 0 : near ? 188 : 42} />
          <BudgetBar label="Size" max={24} unit="KiB" value={empty ? 0 : near ? 23 : 6} />
          {state === "archive" ? (
            <div className="flex items-center gap-2 text-small-body text-muted">
              <Archive aria-hidden="true" className="size-3.5" />
              <span className="min-w-0 flex-1">37 older notes in the archive</span>
              <Button size="xs" type="button" variant="ghost">
                Open
              </Button>
            </div>
          ) : null}
        </EditorCard>
        <EditorCard className="gap-3">
          <span className="flex items-center gap-2 text-small-body font-medium text-fg">
            <History aria-hidden="true" className="size-3.5 text-muted" />
            History
          </span>
          {REVISIONS.map(([when, what]) => (
            <div className="flex items-center gap-2" key={when}>
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="truncate text-small-body text-fg-2">{what}</span>
                <span className="text-eyebrow text-subtle">{when}</span>
              </span>
              <Button size="xs" type="button" variant="ghost">
                Roll back
              </Button>
            </div>
          ))}
        </EditorCard>
      </div>
    </div>
  );
}
