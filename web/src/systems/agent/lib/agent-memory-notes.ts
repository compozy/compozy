import type { AgentMemoryBudget } from "../types";

/** One line of MEMORY.md. */
export interface MemoryNote {
  id: string;
  text: string;
  /** The project it was learned in; absent when the person wrote it. */
  learnedIn?: string;
  /** `YYYY-MM-DD`. */
  date?: string;
  /** `YYYY-MM-DD`, the day it stops being used. */
  until?: string;
  /** Blank lines above this note, kept so a save doesn't reflow the file. */
  gap: number;
  /** The file's line, reused verbatim until the note changes. */
  line?: string;
}

export interface MemoryFile {
  notes: readonly MemoryNote[];
  /** Line breaks after the last note. */
  tail: string;
}

/** A dated note the person wrote; dated lines need a scope, and this one reads as theirs. */
export const OPERATOR_SCOPE = "you";

const LEARNED = /^- (\d{4}-\d{2}-\d{2}) · (.+?) · (.+?)(?: · until (\d{4}-\d{2}-\d{2}))?$/;

function isDate(value: string) {
  const date = new Date(`${value}T00:00:00Z`);
  return !Number.isNaN(date.getTime()) && date.toISOString().startsWith(value);
}

function parseLine(line: string, index: number, gap: number): MemoryNote {
  const id = `line-${index}`;
  const match = LEARNED.exec(line);
  if (!match || !isDate(match[1]!) || (match[4] !== undefined && !isDate(match[4]))) {
    return { id, text: line, gap, line };
  }
  const scope = match[2]!;
  return {
    id,
    text: match[3]!,
    date: match[1]!,
    ...(scope === OPERATOR_SCOPE ? {} : { learnedIn: scope }),
    ...(match[4] ? { until: match[4] } : {}),
    gap,
    line,
  };
}

export function parseMemoryFile(content: string): MemoryFile {
  const tail = /\n*$/.exec(content)?.[0] ?? "";
  const body = content.slice(0, content.length - tail.length);
  const notes: MemoryNote[] = [];
  let gap = 0;
  if (body !== "") {
    body.split("\n").forEach((line, index) => {
      if (line.trim() === "") {
        gap += 1;
        return;
      }
      notes.push(parseLine(line, index, gap));
      gap = 0;
    });
  }
  return { notes, tail };
}

function composeLine(note: MemoryNote) {
  if (note.line !== undefined) return note.line;
  if (!note.date) return note.text;
  const until = note.until ? ` · until ${note.until}` : "";
  return `- ${note.date} · ${note.learnedIn ?? OPERATOR_SCOPE} · ${note.text}${until}`;
}

export function composeMemoryFile({ notes, tail }: MemoryFile) {
  const lines = notes.map(note => `${"\n".repeat(note.gap)}${composeLine(note)}`);
  return `${lines.join("\n")}${notes.length > 0 ? tail || "\n" : ""}`;
}

export function countMemoryChanges(notes: readonly MemoryNote[], saved: readonly MemoryNote[]) {
  const added = notes.filter(note => !saved.some(other => other.id === note.id));
  const removed = saved.filter(note => !notes.some(other => other.id === note.id));
  const edited = notes.filter(note =>
    saved.some(other => other.id === note.id && other.text !== note.text)
  );
  return added.length + removed.length + edited.length;
}

export type MemoryFilter = "all" | "learned" | "yours";

export function memoryNoteMatches(note: MemoryNote, filter: MemoryFilter, search: string) {
  if (filter === "learned" && !note.learnedIn) return false;
  if (filter === "yours" && note.learnedIn) return false;
  const haystack = `${note.text} ${note.learnedIn ?? ""}`.toLowerCase();
  return haystack.includes(search.trim().toLowerCase());
}

export function localDate(date: Date) {
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${date.getFullYear()}-${month}-${day}`;
}

/** Past its `until`: the day itself still counts. */
export function memoryNoteExpired(note: Pick<MemoryNote, "until">, today: string) {
  return note.until !== undefined && note.until < today;
}

const THIS_YEAR = new Intl.DateTimeFormat("en-US", {
  month: "short",
  day: "numeric",
  timeZone: "UTC",
});
const OTHER_YEAR = new Intl.DateTimeFormat("en-US", {
  month: "short",
  day: "numeric",
  year: "numeric",
  timeZone: "UTC",
});

/** `2026-11-02` as `Nov 2`, with the year only when it isn't this one. */
export function formatMemoryDate(value: string, today: string) {
  const format = value.slice(0, 4) === today.slice(0, 4) ? THIS_YEAR : OTHER_YEAR;
  return format.format(new Date(`${value}T00:00:00Z`));
}

export type MemoryFullness = "fine" | "almost" | "full";

const NEAR_BUDGET = 90;

export function memoryPercent(budget: AgentMemoryBudget) {
  const lines = budget.max_lines > 0 ? budget.lines / budget.max_lines : 0;
  const bytes = budget.max_bytes > 0 ? budget.bytes / budget.max_bytes : 0;
  return Math.min(100, Math.round(Math.max(lines, bytes) * 100));
}

/** Full only when nothing can be put away; learned notes make room by themselves. */
export function memoryFullness(percent: number, learned: number): MemoryFullness {
  if (percent >= 100 && learned === 0) return "full";
  return percent >= NEAR_BUDGET ? "almost" : "fine";
}

export function memoryBudgetLine(budget: AgentMemoryBudget) {
  const kb = (bytes: number) => Math.round((bytes / 1024) * 10) / 10;
  return `${budget.lines} of ${budget.max_lines} lines, ${kb(budget.bytes)} of ${kb(budget.max_bytes)} KB used`;
}
