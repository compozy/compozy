/** The changelog view model: one release as the timeline and the release page read it. */
import {
  linkPullRequests,
  parseReleaseNotes,
  type CommitLine,
  type ReleaseExtra,
  type ReleaseNote,
} from "./release-notes";
import type { ChangelogRelease } from "./types";

export { GROUP_ORDER, type CommitLine, type ReleaseExtra, type ReleaseNote } from "./release-notes";

export interface ReleaseView {
  version: string;
  date: string;
  pre: boolean;
  url: string;
  headline: string | null;
  intro: string[];
  notes: ReleaseNote[];
  commits: Record<string, CommitLine[]>;
  extras: ReleaseExtra[];
}

/** What the list needs of one release: small enough to send to the browser for every release. */
export interface TimelineRelease {
  version: string;
  date: string;
  pre: boolean;
  title: string;
  lead: string | null;
  also: string[];
  counts: string[];
  total: number;
  breaking: boolean;
  noted: boolean;
  /** Lower-cased text the search matches against: version, headline, notes and change lines. */
  haystack: string;
}

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

function headlineOf(notes: ReleaseNote[], commits: Record<string, CommitLine[]>): string | null {
  const titled = (group?: string) =>
    notes.find(note => note.title && (group === undefined || note.group === group))?.title;
  return (
    titled("Highlights") ??
    titled("Features") ??
    commits.features?.[0]?.text ??
    titled() ??
    commits.fixes?.[0]?.text ??
    Object.values(commits)[0]?.[0]?.text ??
    null
  );
}

export function toReleaseView(release: ChangelogRelease): ReleaseView {
  const parsed = parseReleaseNotes(release.body);
  const link = (markdown: string) => linkPullRequests(markdown, release.pullRequestNumbers);
  return {
    version: release.version,
    date: release.publishedAt,
    pre: release.prerelease,
    url: release.url,
    headline: headlineOf(parsed.notes, parsed.commits),
    intro: parsed.intro.map(link),
    notes: parsed.notes.map(note => ({
      ...note,
      body: link(note.body),
      ...(note.migration ? { migration: link(note.migration) } : {}),
    })),
    commits: parsed.commits,
    extras: parsed.extras.map(extra => ({ ...extra, body: link(extra.body) })),
  };
}

/** The anchor of a release-note group on the release page. */
export function sectionSlug(group: string) {
  return group.toLowerCase().replace(/\s+/g, "-");
}

export function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  });
}

export function changeCounts(release: ReleaseView) {
  return Object.entries(COMMIT_LABELS).flatMap(([key, [one, many]]) => {
    const count = release.commits[key]?.length ?? 0;
    return count > 0 ? [`${count} ${count === 1 ? one : many}`] : [];
  });
}

export function totalChanges(release: ReleaseView) {
  return Object.values(release.commits).reduce((total, lines) => total + lines.length, 0);
}

export function hasBreaking(release: Pick<ReleaseView, "notes">) {
  return release.notes.some(note => note.group === "Breaking Changes");
}

export function title(release: Pick<ReleaseView, "headline" | "version">) {
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
export function summary(release: ReleaseView) {
  const note = release.notes.find(item => item.title === release.headline);
  if (!note) return null;
  return plain(note.body.split("\n\n")[0] ?? "");
}

export function alsoNew(release: ReleaseView) {
  const fromNotes = release.notes.flatMap(note =>
    note.title && note.title !== release.headline ? [note.title] : []
  );
  const fromCommits = (release.commits.features ?? []).flatMap(line =>
    line.text === release.headline ? [] : [line.text]
  );
  return [...fromNotes, ...fromCommits].slice(0, 3);
}

export function toTimelineRelease(release: ReleaseView): TimelineRelease {
  return {
    version: release.version,
    date: release.date,
    pre: release.pre,
    title: title(release),
    lead: summary(release),
    also: alsoNew(release),
    counts: changeCounts(release),
    total: totalChanges(release),
    breaking: hasBreaking(release),
    noted: release.notes.length > 0,
    haystack: [
      release.version,
      release.headline ?? "",
      ...release.intro,
      ...release.notes.flatMap(note => [note.title ?? "", note.body]),
      ...Object.values(release.commits)
        .flat()
        .map(line => line.text),
    ]
      .join(" ")
      .toLowerCase(),
  };
}

export function matchesSearch(release: TimelineRelease, needle: string) {
  return release.haystack.includes(needle);
}

export function isMajor(release: Pick<ReleaseView, "version">) {
  return /^v\d+\.\d+\.0$/.test(release.version);
}

export interface Era<T> {
  key: string;
  releases: T[];
  range: string;
  beta: boolean;
}

export function minorOf(version: string) {
  return /^v(\d+\.\d+)/.exec(version)?.[1] ?? version;
}

export function monthLabel(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  });
}

/**
 * Cuts the line into eras: runs of consecutive releases on the same minor version. The input
 * order is kept, so betas published between two stable releases form their own era there.
 */
export function groupEras<T extends { version: string; date: string; pre: boolean }>(
  list: readonly T[]
): Era<T>[] {
  const runs: Array<{ key: string; releases: T[] }> = [];
  for (const release of list) {
    const key = minorOf(release.version);
    const last = runs[runs.length - 1];
    if (last && last.key === key) last.releases.push(release);
    else runs.push({ key, releases: [release] });
  }
  return runs.map(({ key, releases }) => {
    const newest = monthLabel(releases[0]!.date);
    const oldest = monthLabel(releases[releases.length - 1]!.date);
    return {
      key,
      releases,
      range: newest === oldest ? newest : `${oldest} to ${newest}`,
      beta: releases.every(release => release.pre),
    };
  });
}

/** The beta side branch as the list summarises it: how many betas, over which months. */
export function betaBranch(list: readonly TimelineRelease[]) {
  const betas = list.filter(release => release.pre);
  const newest = betas[0];
  const oldest = betas[betas.length - 1];
  if (!newest || !oldest) return { count: 0, range: "" };
  const month = (iso: string) =>
    new Date(iso).toLocaleDateString("en-US", { month: "short", timeZone: "UTC" });
  const year = (iso: string) => new Date(iso).getUTCFullYear();
  const range =
    year(oldest.date) === year(newest.date)
      ? `${month(oldest.date)} to ${month(newest.date)} ${year(newest.date)}`
      : `${monthLabel(oldest.date)} to ${monthLabel(newest.date)}`;
  return { count: betas.length, range };
}
