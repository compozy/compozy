/** One note as a readable card, its inline editor, and the composer for a new one. */
import { CalendarClock, MapPin, Pencil, Trash2, UserRound } from "lucide-react";
import { useState } from "react";

import { Button, DateTimePicker, MarkdownEditor, Pill, cn } from "@compozy/ui";

import { EditorCard } from "../persistent-agents-shared/rich-controls";
import type { Note } from "./memory-fixtures";

const MAX_NOTE = 1000;

function Source({ note }: { note: Note }) {
  const Icon = note.learnedIn ? MapPin : UserRound;
  return (
    <span className="flex min-w-0 flex-1 flex-wrap items-center gap-x-1.5 gap-y-0.5 text-eyebrow text-subtle">
      <Icon aria-hidden="true" className="size-3 shrink-0" />
      <span className="min-w-0 truncate">
        {note.learnedIn ? `Learned in ${note.learnedIn}` : "You wrote this"}
      </span>
      {note.date ? <span>· {note.date}</span> : null}
    </span>
  );
}

function Forgets({ note }: { note: Note }) {
  if (!note.until) return null;
  return (
    <span className="flex items-center gap-1.5 text-eyebrow text-subtle">
      <CalendarClock aria-hidden="true" className="size-3 shrink-0" />
      {note.expired ? `Stopped using this on ${note.until}` : `Forgets this on ${note.until}`}
    </span>
  );
}

/** Note text with inline `code` drawn the way the Markdown editor draws it. */
function NoteText({ text, expired }: { text: string; expired?: boolean }) {
  return (
    <p className={cn("text-body leading-relaxed", expired ? "text-subtle" : "text-fg")}>
      {text.split(/(`[^`]+`)/).map(part =>
        part.startsWith("`") ? (
          <code className="rounded-xxs bg-surface-2 px-1 font-mono text-[0.92em]" key={part}>
            {part.slice(1, -1)}
          </code>
        ) : (
          part
        )
      )}
    </p>
  );
}

export function NoteComposer({
  initial = "",
  action,
  onCancel,
  onDone,
}: {
  initial?: string;
  action: string;
  onCancel: () => void;
  onDone: (text: string) => void;
}) {
  const [text, setText] = useState(initial);
  const [until, setUntil] = useState<Date | null>(null);
  const over = text.length > MAX_NOTE;
  const ready = text.trim() !== "" && !over;
  return (
    <div className="flex flex-col gap-3">
      <MarkdownEditor
        aria-label="Note"
        aria-invalid={over}
        autoFocus
        onSubmit={() => (ready ? onDone(text.trim()) : undefined)}
        onValueChange={setText}
        placeholder="Something it should always remember"
        value={text}
      />
      <div className="flex items-center gap-2">
        <div className="w-44">
          <DateTimePicker
            clearLabel="Keep it forever"
            onValueChange={setUntil}
            placeholder="Forget on a date"
            value={until}
          />
        </div>
        <span
          className={cn(
            "ml-auto text-eyebrow tabular-nums",
            over ? "font-medium text-danger" : "text-subtle"
          )}
        >
          {text.length.toLocaleString("en-US")} / 1,000
        </span>
        <Button onClick={onCancel} size="sm" type="button" variant="ghost">
          Cancel
        </Button>
        <Button disabled={!ready} onClick={() => onDone(text.trim())} size="sm" type="button">
          {action}
        </Button>
      </div>
    </div>
  );
}

export function NoteCard({
  note,
  pending,
  onEdit,
  onRemove,
}: {
  note: Note;
  /** Added or changed here and not saved yet. */
  pending: boolean;
  onEdit: (text: string) => void;
  onRemove: () => void;
}) {
  const [editing, setEditing] = useState(false);
  return (
    <li className="mb-3 break-inside-avoid">
      <EditorCard className="gap-3 p-4">
        {editing ? (
          <NoteComposer
            action="Done"
            initial={note.text}
            onCancel={() => setEditing(false)}
            onDone={text => {
              onEdit(text);
              setEditing(false);
            }}
          />
        ) : (
          <>
            <NoteText expired={note.expired} text={note.text} />
            <Forgets note={note} />
            <div className="flex min-h-6 items-center gap-1">
              <Source note={note} />
              {pending ? (
                <Pill form="plain" size="xs">
                  Not saved yet
                </Pill>
              ) : null}
              <Button
                aria-label="Edit note"
                onClick={() => setEditing(true)}
                size="icon-xs"
                type="button"
                variant="quiet"
              >
                <Pencil aria-hidden="true" />
              </Button>
              <Button
                aria-label="Remove note"
                onClick={onRemove}
                size="icon-xs"
                type="button"
                variant="quiet"
              >
                <Trash2 aria-hidden="true" />
              </Button>
            </div>
          </>
        )}
      </EditorCard>
    </li>
  );
}
