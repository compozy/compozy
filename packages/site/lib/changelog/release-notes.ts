/**
 * Splits a published GitHub release body into the parts the changelog shows: the written
 * release notes (group, title, body, migration), the merged-change lines with their pull
 * requests, and any other section kept verbatim. Nothing in the body is dropped.
 */
import { stripLeadingReleaseHeading } from "./release-markdown";
import { COMPOZY_REPOSITORY } from "./types";

export interface ReleaseNote {
  group: string;
  title: string | null;
  body: string;
  migration?: string;
}

export interface CommitLine {
  text: string;
  prs: string[];
}

export interface ReleaseExtra {
  heading: string;
  body: string;
}

export interface ParsedReleaseNotes {
  /** Untitled prose: the paragraph before the first section and before the first note group. */
  intro: string[];
  notes: ReleaseNote[];
  commits: Record<string, CommitLine[]>;
  extras: ReleaseExtra[];
}

/** The order release-note groups read in. Groups outside it follow, in body order. */
export const GROUP_ORDER = ["Highlights", "Breaking Changes", "Features", "Improvements", "Fixes"];

const COMMIT_SECTIONS: Record<string, string> = {
  "bug fixes": "fixes",
  features: "features",
  "performance improvements": "performance",
  refactoring: "refactors",
  testing: "tests",
  documentation: "docs",
  "miscellaneous tasks": "chores",
  build: "build",
};

const HEADING = /^(#{2,6})\s+(.+?)\s*#*\s*$/;
const TRAILING_PRS = /\s*\(((?:#\d+(?:,\s*)?)+)\)\s*$/;

function label(input: string): string {
  return input.replace(/^[^\p{L}\p{N}`*]+/u, "").trim();
}

type Sink =
  | { kind: "none" }
  | { kind: "intro" }
  | { kind: "commits"; key: string }
  | { kind: "note"; note: ReleaseNote }
  | { kind: "migration"; note: ReleaseNote }
  | { kind: "extra"; extra: ReleaseExtra };

function appendLine(text: string | undefined, line: string): string {
  return text === undefined || text === "" ? line : `${text}\n${line}`;
}

function commitLine(raw: string): CommitLine {
  const match = TRAILING_PRS.exec(raw);
  if (!match) return { text: raw.trim(), prs: [] };
  const prs = Array.from(match[1]?.matchAll(/#(\d+)/g) ?? [], found => found[1] ?? "");
  return { text: raw.slice(0, match.index).trim(), prs: prs.filter(Boolean) };
}

function groupRank(group: string): number {
  const index = GROUP_ORDER.indexOf(group);
  return index === -1 ? GROUP_ORDER.length : index;
}

export function parseReleaseNotes(body: string): ParsedReleaseNotes {
  const notes: ReleaseNote[] = [];
  const commits: Record<string, CommitLine[]> = {};
  const extras: ReleaseExtra[] = [];
  const intro: string[] = [""];
  let inNotes = false;
  let group: string | null = null;
  let sink = { kind: "none" } as Sink;
  let fenced = false;

  const openExtra = (heading: string) => {
    const extra = { heading, body: "" };
    extras.push(extra);
    sink = { kind: "extra", extra };
  };

  const write = (line: string) => {
    switch (sink.kind) {
      case "commits": {
        const lines = commits[sink.key] ?? [];
        commits[sink.key] = lines;
        const bullet = /^\s*[-*+]\s+(.*)$/.exec(line);
        if (bullet) lines.push(commitLine(bullet[1] ?? ""));
        else if (line.trim() && lines.length > 0) {
          const last = lines[lines.length - 1]!;
          lines[lines.length - 1] = commitLine(
            `${last.text} ${line.trim()}${last.prs.length ? ` (${last.prs.map(pr => `#${pr}`).join(", ")})` : ""}`
          );
        }
        return;
      }
      case "note":
        sink.note.body = appendLine(sink.note.body, line);
        return;
      case "migration":
        sink.note.migration = appendLine(sink.note.migration, line);
        return;
      case "extra":
        sink.extra.body = appendLine(sink.extra.body, line);
        return;
      case "intro":
        intro[intro.length - 1] = appendLine(intro[intro.length - 1], line);
        return;
      case "none":
        if (!line.trim()) return;
        if (inNotes && group) {
          const note: ReleaseNote = { group, title: null, body: line };
          notes.push(note);
          sink = { kind: "note", note };
          return;
        }
        intro.push(line);
        sink = { kind: "intro" };
        return;
    }
  };

  for (const line of stripLeadingReleaseHeading(body).split("\n")) {
    if (/^\s*(```|~~~)/.test(line)) fenced = !fenced;
    const heading = fenced ? null : HEADING.exec(line.trim());
    if (!heading || /^\s*(```|~~~)/.test(line)) {
      write(line);
      continue;
    }
    const level = heading[1]?.length ?? 0;
    const text = label(heading[2] ?? "");
    const lastNote: ReleaseNote | null =
      sink.kind === "note" || sink.kind === "migration" ? sink.note : null;

    if (level <= 3 && /^migration( notes)?$/i.test(text) && lastNote) {
      sink = { kind: "migration", note: lastNote };
      if (lastNote.migration) lastNote.migration = `${lastNote.migration}\n`;
      continue;
    }
    if (level <= 3) {
      if (/^release notes$/i.test(text)) {
        inNotes = true;
        group = null;
        sink = { kind: "none" };
        intro.push("");
        continue;
      }
      const key = inNotes ? undefined : COMMIT_SECTIONS[text.toLowerCase()];
      if (key) sink = { kind: "commits", key };
      else openExtra(text);
      continue;
    }
    if (inNotes && sink.kind !== "extra" && level === 4) {
      group = text;
      sink = { kind: "none" };
      continue;
    }
    if (inNotes && sink.kind !== "extra" && level === 5 && group) {
      const note: ReleaseNote = { group, title: text, body: "" };
      notes.push(note);
      sink = { kind: "note", note };
      continue;
    }
    write(line);
  }

  const kept: ReleaseNote[] = [];
  for (const note of notes) {
    const text = note.body.trim();
    if (!note.title && !text) continue;
    const migration = note.migration?.trim();
    kept.push({
      group: note.group,
      title: note.title,
      body: text,
      ...(migration ? { migration } : {}),
    });
  }
  const keptExtras: ReleaseExtra[] = [];
  for (const extra of extras) {
    const text = extra.body.trim();
    if (text) keptExtras.push({ heading: extra.heading, body: text });
  }
  return {
    intro: intro.flatMap(text => (text.trim() ? [text.trim()] : [])),
    notes: kept.toSorted((left, right) => groupRank(left.group) - groupRank(right.group)),
    commits: Object.fromEntries(Object.entries(commits).filter(([, lines]) => lines.length > 0)),
    extras: keptExtras,
  };
}

/** Links bare `#123` references to their pull requests, outside code and existing links. */
export function linkPullRequests(markdown: string, pullRequestNumbers: readonly number[]): string {
  const known = new Set(pullRequestNumbers);
  if (known.size === 0) return markdown;
  let fenced = false;
  return markdown
    .split("\n")
    .map(line => {
      if (/^\s*(```|~~~)/.test(line)) {
        fenced = !fenced;
        return line;
      }
      if (fenced) return line;
      return line
        .split(/(`+[^`]*`+|\[[^\]]*\]\([^)]*\))/)
        .map((part, index) =>
          index % 2 === 1
            ? part
            : part.replace(/(^|[^\w/[&])#(\d{1,7})\b/g, (whole, before: string, digits: string) =>
                known.has(Number(digits))
                  ? `${before}[#${digits}](${COMPOZY_REPOSITORY.url}/pull/${digits})`
                  : whole
              )
        )
        .join("");
    })
    .join("\n");
}
