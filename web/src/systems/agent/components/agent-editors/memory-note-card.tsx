/** One note as a sheet of paper on the board, its inline editor, and the composer for a new one. */
import { CalendarClock, Gavel, NotebookPen, Pencil, Trash2, UserRound } from "@compozy/ui/icons";
import { useState, type CSSProperties, type ReactNode } from "react";

import { Button, DateTimePicker, MarkdownEditor, Pill, PillGroup, cn } from "@compozy/ui";

import {
  formatMemoryDate,
  isMemoryRule,
  localDate,
  memoryNoteExpired,
  type MemoryKind,
  type MemoryNote,
} from "../../lib/agent-memory-notes";
import { agentEditorMark } from "../../lib/agent-editor-mark";
import { agentScopeLabel } from "../../lib/agent-instance";
import type { AgentPayload } from "../../types";
import { AgentMark } from "../agent-mark";
import { InlineCodeText } from "../inline-code-text";

const MAX_NOTE = 1000;

const KINDS: readonly { value: MemoryKind; label: string }[] = [
  { value: "note", label: "Note" },
  { value: "rule", label: "Rule" },
];

const RULE_HELP = "The agent follows rules on every turn. Only you can change them.";

function Source({
  agent,
  note,
  today,
  scopeLabel,
}: {
  agent: AgentPayload;
  note: MemoryNote;
  today: string;
  scopeLabel: (scope: string) => string;
}) {
  return (
    <span className="flex min-w-0 flex-1 items-center gap-1.5 text-eyebrow text-muted-foreground">
      {note.learnedIn ? (
        <AgentMark {...agentEditorMark(agent)} quietIdle size={18} />
      ) : (
        <UserRound aria-hidden="true" className="size-3.5 shrink-0" />
      )}
      <span className="min-w-0 truncate">
        {note.learnedIn ? `Learned in ${scopeLabel(note.learnedIn)}` : "You wrote this"}
        {note.date ? ` · ${formatMemoryDate(note.date, today)}` : ""}
      </span>
    </span>
  );
}

function Forgets({ note, today }: { note: MemoryNote; today: string }) {
  if (!note.until) return null;
  const date = formatMemoryDate(note.until, today);
  return (
    <span className="flex items-center gap-1.5 text-eyebrow text-muted-foreground">
      <CalendarClock aria-hidden="true" className="size-3 shrink-0" />
      {memoryNoteExpired(note, today) ? `Stopped using this on ${date}` : `Forgets this on ${date}`}
    </span>
  );
}

/** Note text with inline `code`. */
function NoteText({ text, expired }: { text: string; expired?: boolean }) {
  return (
    <p className={cn("text-body leading-relaxed", expired ? "text-subtle" : "text-fg")}>
      <InlineCodeText text={text} />
    </p>
  );
}

export function NoteComposer({
  initial = "",
  initialKind = "note",
  chooseKind = false,
  action,
  onCancel,
  onDone,
}: {
  initial?: string;
  initialKind?: MemoryKind;
  /** Offer Note or Rule; an entry being edited keeps its kind. */
  chooseKind?: boolean;
  action: string;
  onCancel: () => void;
  onDone: (text: string, until?: string, kind?: MemoryKind) => void;
}) {
  const [text, setText] = useState(initial);
  const [kind, setKind] = useState<MemoryKind>(initialKind);
  const rule = kind === "rule";
  const [until, setUntil] = useState<Date | null>(null);
  const over = text.length > MAX_NOTE;
  const ready = text.trim() !== "" && !over;
  const done = () => onDone(text.trim(), !rule && until ? localDate(until) : undefined, kind);
  return (
    <div className="flex flex-col gap-3">
      {chooseKind ? (
        <PillGroup aria-label="Kind" items={KINDS} onChange={setKind} size="sm" value={kind} />
      ) : null}
      <MarkdownEditor
        aria-label={rule ? "Rule" : "Note"}
        aria-invalid={over}
        autoFocus
        onSubmit={() => (ready ? done() : undefined)}
        onValueChange={setText}
        placeholder={rule ? "Something it must always do" : "Something it should always remember"}
        value={text}
      />
      <div className="flex flex-wrap items-center gap-2">
        {rule ? null : (
          <div className="w-44">
            <DateTimePicker
              clearLabel="Keep it forever"
              onValueChange={setUntil}
              placeholder="Forget on a date"
              value={until}
            />
          </div>
        )}
        <span
          className={cn(
            "ml-auto text-eyebrow tabular-nums",
            over ? "font-medium text-destructive" : "text-subtle"
          )}
        >
          {text.length.toLocaleString("en-US")} / 1,000
        </span>
        <Button onClick={onCancel} size="sm" type="button" variant="ghost">
          Cancel
        </Button>
        <Button disabled={!ready} onClick={done} size="sm" type="button">
          {chooseKind && rule ? "Add rule" : action}
        </Button>
      </div>
      {rule ? <p className="text-eyebrow text-muted-foreground">{RULE_HELP}</p> : null}
    </div>
  );
}

const TILTS = [-1.1, 0.7, -0.5, 1, -0.8, 0.4];

/** A note keeps the same tilt through every render, whatever else is on the board. */
function tiltOf(id: string) {
  const sum = [...id].reduce((total, char) => total + char.charCodeAt(0), 0);
  return TILTS[sum % TILTS.length] ?? 0;
}

/** Yours and learned read as two pads of paper; the same pair the memory meter names. */
function paper(note: Pick<MemoryNote, "learnedIn">, tilt: number) {
  return {
    "--note-paper": note.learnedIn
      ? "color-mix(in srgb, var(--color-fg) 7%, var(--color-card))"
      : "color-mix(in srgb, var(--color-info) 20%, var(--color-card))",
    "--note-tilt": `${tilt}deg`,
    backgroundColor: "var(--note-paper)",
  } as CSSProperties;
}

/** The sheet itself: a strip of tape at the top and one corner lifting off the board. */
export function NoteSheet({
  note = {},
  tilt = 0,
  faded = false,
  className,
  children,
}: {
  note?: Pick<MemoryNote, "learnedIn">;
  tilt?: number;
  faded?: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div
      className={cn(
        "group/note relative flex min-h-40 flex-col gap-3 rounded-xs p-4 pt-5 shadow-card",
        "[rotate:var(--note-tilt)] transition-[rotate,box-shadow] duration-base hover:z-10 hover:[rotate:0deg] hover:shadow-overlay focus-within:z-10 focus-within:[rotate:0deg] focus-within:shadow-overlay motion-reduce:transition-none",
        faded && "opacity-75 hover:opacity-100 focus-within:opacity-100",
        className
      )}
      style={paper(note, tilt)}
    >
      <span
        aria-hidden="true"
        className="absolute -top-2 left-1/2 h-4 w-16 -translate-x-1/2 -rotate-2 bg-viz-bar opacity-50"
      />
      {children}
      <span
        aria-hidden="true"
        className="absolute right-0 bottom-0 size-4"
        style={{
          background:
            "linear-gradient(135deg, color-mix(in srgb, var(--note-paper) 86%, var(--color-fg)) 50%, var(--color-sunken) 50%)",
        }}
      />
    </div>
  );
}

export function NoteCard({
  agent,
  note,
  today,
  pending,
  scopeLabel = agentScopeLabel,
  onEdit,
  onKind,
  onRemove,
}: {
  agent: AgentPayload;
  note: MemoryNote;
  today: string;
  /** Notes keep the id they were learned under; the page shows its name. */
  scopeLabel?: (scope: string) => string;
  /** Added or changed here and not saved yet. */
  pending: boolean;
  onEdit: (text: string) => void;
  /** Switches the entry between note and rule. */
  onKind: () => void;
  onRemove: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const expired = memoryNoteExpired(note, today);
  const rule = isMemoryRule(note);
  const name = rule ? "rule" : "note";
  return (
    <li>
      <NoteSheet faded={expired && !editing} note={note} tilt={editing ? 0 : tiltOf(note.id)}>
        {editing ? (
          <NoteComposer
            action="Done"
            initial={note.text}
            initialKind={rule ? "rule" : "note"}
            onCancel={() => setEditing(false)}
            onDone={text => {
              onEdit(text);
              setEditing(false);
            }}
          />
        ) : (
          <>
            <NoteText expired={expired} text={note.text} />
            <div className="mt-auto flex flex-col gap-1.5 pt-1">
              <Forgets note={note} today={today} />
              <div className="flex min-h-6 items-center gap-1 pr-3">
                <Source agent={agent} note={note} scopeLabel={scopeLabel} today={today} />
                {rule ? (
                  <Pill size="xs" title={RULE_HELP} tone="info">
                    Rule
                  </Pill>
                ) : null}
                {pending ? (
                  <Pill form="plain" size="xs">
                    Not saved yet
                  </Pill>
                ) : null}
              </div>
            </div>
            <div
              className="absolute top-2 right-2 flex gap-0.5 rounded-sm opacity-0 transition-opacity duration-base group-hover/note:opacity-100 group-focus-within/note:opacity-100 motion-reduce:transition-none [@media(hover:none)]:opacity-100"
              style={{ backgroundColor: "var(--note-paper)" }}
            >
              <Button
                aria-label={rule ? "Make it a note" : "Make it a rule"}
                onClick={onKind}
                size="icon-xs"
                title={rule ? "Make it a note" : "Make it a rule"}
                type="button"
                variant="quiet"
              >
                {rule ? <NotebookPen aria-hidden="true" /> : <Gavel aria-hidden="true" />}
              </Button>
              <Button
                aria-label={`Edit ${name}`}
                onClick={() => setEditing(true)}
                size="icon-xs"
                type="button"
                variant="quiet"
              >
                <Pencil aria-hidden="true" />
              </Button>
              <Button
                aria-label={`Remove ${name}`}
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
      </NoteSheet>
    </li>
  );
}

/** Splits notes into two even piles, so a long note never leaves a hole beside it. */
function piles(notes: readonly MemoryNote[]) {
  const left: MemoryNote[] = [];
  const right: MemoryNote[] = [];
  let leftWeight = 0;
  let rightWeight = 0;
  for (const note of notes) {
    const weight = note.text.length + 90;
    if (leftWeight <= rightWeight) {
      left.push(note);
      leftWeight += weight;
    } else {
      right.push(note);
      rightWeight += weight;
    }
  }
  return [left, right];
}

const BOARD_DOTS: CSSProperties = {
  backgroundImage: "radial-gradient(var(--color-line) 1px, transparent 1px)",
  backgroundSize: "16px 16px",
};

/** The surface notes are stuck to. */
export function NoteBoard({ children }: { children: ReactNode }) {
  return (
    <div className="@container rounded-lg bg-sunken p-5 shadow-card" style={BOARD_DOTS}>
      {children}
    </div>
  );
}

export function NotePiles({
  notes,
  render,
}: {
  notes: readonly MemoryNote[];
  render: (note: MemoryNote) => ReactNode;
}) {
  return (
    <div className="grid grid-cols-1 items-start gap-5 @lg:grid-cols-2">
      {piles(notes).map(pile => (
        <ul className="flex min-w-0 flex-col gap-5" key={pile[0]?.id ?? "empty"}>
          {pile.map(render)}
        </ul>
      ))}
    </div>
  );
}
