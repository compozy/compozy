/** A real snapshot of every GitHub release. Production reads these from the changelog client. */
import raw from "./releases.json";

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

export interface Release {
  version: string;
  date: string;
  pre: boolean;
  url: string;
  headline: string | null;
  notes: ReleaseNote[];
  commits: Record<string, CommitLine[]>;
  extras: Array<{ heading: string; body: string }>;
}

export const RELEASES = raw as Release[];
export const REPO = "https://github.com/compozy/compozy";

export const GROUP_ORDER = ["Highlights", "Breaking Changes", "Features", "Improvements", "Fixes"];

const COMMIT_LABELS: Record<string, readonly [string, string]> = {
  features: ["feature", "features"],
  fixes: ["fix", "fixes"],
  performance: ["performance change", "performance changes"],
};

export const COMMIT_TITLES: Record<string, string> = {
  features: "Features",
  fixes: "Fixes",
  performance: "Performance",
  refactors: "Refactoring",
  tests: "Testing",
  docs: "Documentation",
  chores: "Chores",
  build: "Build",
};

export function findRelease(version: string) {
  return RELEASES.find(release => release.version === version);
}

export function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  });
}

export function changeCounts(release: Release) {
  return Object.entries(COMMIT_LABELS).flatMap(([key, [one, many]]) => {
    const count = release.commits[key]?.length ?? 0;
    return count > 0 ? [`${count} ${count === 1 ? one : many}`] : [];
  });
}

export function totalChanges(release: Release) {
  return Object.values(release.commits).reduce((total, lines) => total + lines.length, 0);
}

export function hasBreaking(release: Release) {
  return release.notes.some(note => note.group === "Breaking Changes");
}

export function title(release: Release) {
  return release.headline ?? `Release ${release.version}`;
}

function plain(markdown: string) {
  return markdown
    .replace(/\*\*([^*]+)\*\*/g, "$1")
    .replace(/`([^`]+)`/g, "$1")
    .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
    .replace(/^- /, "");
}

/** The first paragraph of the note the headline came from, when one was written for people. */
export function summary(release: Release) {
  const note = release.notes.find(item => item.title === release.headline);
  if (!note) return null;
  return plain(note.body.split("\n\n")[0] ?? "");
}

export function alsoNew(release: Release) {
  const fromNotes = release.notes.flatMap(note =>
    note.title && note.title !== release.headline ? [note.title] : []
  );
  const fromCommits = (release.commits.features ?? []).flatMap(line =>
    line.text === release.headline ? [] : [line.text]
  );
  return [...fromNotes, ...fromCommits].slice(0, 3);
}

export function matchesSearch(release: Release, needle: string) {
  const haystack = [
    release.version,
    release.headline ?? "",
    ...release.notes.flatMap(note => [note.title ?? "", note.body]),
    ...Object.values(release.commits)
      .flat()
      .map(line => line.text),
  ];
  return haystack.join(" ").toLowerCase().includes(needle);
}

export interface Era {
  key: string;
  releases: Release[];
  range: string;
  beta: boolean;
}

function minorOf(version: string) {
  return /^v(\d+\.\d+)/.exec(version)?.[1] ?? version;
}

function monthLabel(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  });
}

export function groupEras(list: readonly Release[]): Era[] {
  const eras = new Map<string, Release[]>();
  for (const release of list) {
    const key = minorOf(release.version);
    eras.set(key, [...(eras.get(key) ?? []), release]);
  }
  return Array.from(eras, ([key, releases]) => {
    const newest = monthLabel(releases[0].date);
    const oldest = monthLabel(releases[releases.length - 1].date);
    return {
      key,
      releases,
      range: newest === oldest ? newest : `${oldest} to ${newest}`,
      beta: releases.every(release => release.pre),
    };
  });
}

export const BETAS = RELEASES.filter(release => release.pre);

export function isMajor(release: Release) {
  return release.version.endsWith(".0");
}
