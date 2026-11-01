import { useState } from "react";

import { toast } from "@compozy/ui";

import { EXPIRED_NOTES, NOTES, THEIR_NOTE, type Note } from "../memory-fixtures";
import type { Fullness } from "../memory-side";

export type MemoryVariant =
  | "filled"
  | "empty"
  | "adding"
  | "unsaved"
  | "conflict"
  | "almost-full"
  | "full"
  | "expired"
  | "advanced";

export type Filter = "all" | "learned" | "yours";

const ALL_YOURS: readonly Note[] = NOTES.map(({ id, text }) => ({ id, text }));

function savedNotes(variant: MemoryVariant): readonly Note[] {
  if (variant === "empty") return [];
  if (variant === "expired") return EXPIRED_NOTES;
  return variant === "full" ? ALL_YOURS : NOTES;
}

function startingNotes(variant: MemoryVariant) {
  const notes = savedNotes(variant);
  if (variant !== "unsaved" && variant !== "conflict") return notes;
  return [
    { id: "draft", text: "Ana is out until Nov 10. Send design reviews to Bruno." },
    ...notes.filter(note => note.id !== "flaky"),
  ];
}

function changeCount(notes: readonly Note[], saved: readonly Note[]) {
  const added = notes.filter(note => !saved.some(other => other.id === note.id));
  const removed = saved.filter(note => !notes.some(other => other.id === note.id));
  const edited = notes.filter(note =>
    saved.some(other => other.id === note.id && other.text !== note.text)
  );
  return added.length + removed.length + edited.length;
}

const FULLNESS: Partial<Record<MemoryVariant, { percent: number; fullness: Fullness }>> = {
  "almost-full": { percent: 94, fullness: "almost" },
  full: { percent: 100, fullness: "full" },
};

function matches(note: Note, filter: Filter, search: string) {
  if (filter === "learned" && !note.learnedIn) return false;
  if (filter === "yours" && note.learnedIn) return false;
  const haystack = `${note.text} ${note.learnedIn ?? ""}`.toLowerCase();
  return haystack.includes(search.trim().toLowerCase());
}

/** Memory's notes against what's saved, with search, adding and rollback. */
export function useMemoryEditor(variant: MemoryVariant) {
  const [saved, setSaved] = useState(() => savedNotes(variant));
  const [notes, setNotes] = useState(() => startingNotes(variant));
  const [raw, setRaw] = useState<string | null>(null);
  const [adding, setAdding] = useState(variant === "adding");
  const [seed, setSeed] = useState("");
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const [conflict, setConflict] = useState(variant === "conflict");
  const [restored, setRestored] = useState<string | null>(null);

  const fixed = FULLNESS[variant];
  const fullness = fixed?.fullness ?? "fine";

  return {
    notes,
    saved,
    raw,
    adding,
    seed,
    search,
    filter,
    conflict,
    restored,
    percent: fixed?.percent ?? Math.round((notes.length / 38) * 100),
    fullness,
    full: fullness === "full" && notes.length >= saved.length,
    learned: notes.filter(note => note.learnedIn).length,
    shown: notes.filter(note => matches(note, filter, search)),
    changes: changeCount(notes, saved) + (raw === null ? 0 : 1),
    setRaw,
    setSearch,
    setFilter,
    startAdding: (text = "") => {
      setSeed(text);
      setAdding(true);
    },
    cancelAdding: () => setAdding(false),
    add: (text: string) => {
      setNotes(current => [{ id: crypto.randomUUID(), text }, ...current]);
      setAdding(false);
      setSeed("");
      setFilter("all");
      setSearch("");
    },
    update: (id: string, text: string) =>
      setNotes(current => current.map(note => (note.id === id ? { ...note, text } : note))),
    remove: (id: string) => setNotes(current => current.filter(note => note.id !== id)),
    keepMine: () => {
      setConflict(false);
      toast("Kept your version");
    },
    useTheirs: () => {
      const theirs = [THEIR_NOTE, ...NOTES];
      setSaved(theirs);
      setNotes(theirs);
      setConflict(false);
    },
    restore: (id: string, keep: number) => {
      setNotes(saved.slice(saved.length - keep));
      setRestored(id);
    },
    discard: () => {
      setNotes(saved);
      setRaw(null);
      setRestored(null);
    },
    save: () => {
      setSaved(notes);
      setRaw(null);
      setRestored(null);
      toast("Saved. It takes effect from the next session.");
    },
  };
}
