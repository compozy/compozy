/** The changelog view model: one release as the timeline and the release page read it. */
import {
  linkPullRequests,
  parseReleaseNotes,
  type CommitLine,
  type ReleaseExtra,
  type ReleaseNote,
} from "./release-notes";
import { CHANGELOG_BASE_TAG, type ChangelogRelease } from "./types";

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
 * Cuts the line into eras: runs of consecutive releases on the same minor version and channel. The
 * input order is kept, so betas published between two stable releases form their own era there,
 * even when they share a minor version with those stable releases.
 */
export function groupEras<T extends Lineable>(list: readonly T[]): Era<T>[] {
  const runs: Array<{ key: string; releases: T[] }> = [];
  for (const release of list) {
    const key = minorOf(release.version);
    const last = runs[runs.length - 1];
    if (last && last.key === key && last.releases[0]!.pre === release.pre) {
      last.releases.push(release);
    } else runs.push({ key, releases: [release] });
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

interface Lineable {
  version: string;
  date: string;
  pre: boolean;
}

/** A run of consecutive betas on the line, folded into one row of the Stable view. */
export interface BetaRun {
  /** The minor version the newest beta was cut for: "0.3". */
  minor: string;
  count: number;
  /** The months the run spans: "Jul to Oct 2026". */
  range: string;
  /** The stable release the betas led to; `null` while it has not shipped. */
  target: string | null;
  /** The target is the first stable release on the line: CompozyOS went stable there. */
  firstStable: boolean;
  /** The stable release the betas branched off. */
  after: string;
}

export type LineItem<T> = { kind: "era"; era: Era<T> } | { kind: "betas"; run: BetaRun };

function coreOf(version: string) {
  const match = /^v?(\d+)\.(\d+)\.(\d+)/.exec(version);
  return match ? [Number(match[1]), Number(match[2]), Number(match[3])] : null;
}

/** Whether `stable` ships at or after the version the beta was cut for. */
function reaches(stable: string, beta: string) {
  const [to, from] = [coreOf(stable), coreOf(beta)];
  if (!to || !from) return false;
  const order = to.findIndex((part, index) => part !== from[index]);
  return order === -1 || to[order]! > from[order]!;
}

function monthRange(newestIso: string, oldestIso: string) {
  const month = (iso: string) =>
    new Date(iso).toLocaleDateString("en-US", { month: "short", timeZone: "UTC" });
  const year = (iso: string) => new Date(iso).getUTCFullYear();
  if (monthLabel(newestIso) === monthLabel(oldestIso)) return monthLabel(newestIso);
  return year(oldestIso) === year(newestIso)
    ? `${month(oldestIso)} to ${month(newestIso)} ${year(newestIso)}`
    : `${monthLabel(oldestIso)} to ${monthLabel(newestIso)}`;
}

/**
 * The run of betas starting at `start` (newest first): the first stable release published after it
 * that reaches the betas' version is the one they led to, so a later patch of an older minor never
 * claims them. A run that reaches the end of the line branched off the
 * release before the cutoff.
 */
function betaRunAt<T extends Lineable>(list: readonly T[], start: number) {
  let end = start;
  while (end + 1 < list.length && list[end + 1]!.pre) end += 1;
  const newest = list[start]!;
  const oldest = list[end]!;
  let target = -1;
  for (let index = start - 1; index >= 0; index -= 1) {
    const candidate = list[index]!;
    if (!candidate.pre && reaches(candidate.version, newest.version)) {
      target = index;
      break;
    }
  }
  const before = list[end + 1];
  const run: BetaRun = {
    minor: minorOf(newest.version),
    count: end - start + 1,
    range: monthRange(newest.date, oldest.date),
    target: target === -1 ? null : list[target]!.version,
    firstStable: target !== -1 && list.slice(target + 1).every(release => release.pre),
    after: before?.version ?? CHANGELOG_BASE_TAG,
  };
  return { run, end };
}

/**
 * The Stable view of the line, newest first: stable releases grouped into eras, and every run of
 * betas folded into one item at its place. `limit` caps the stable releases shown; when it cuts an
 * era, a fold that follows that era still shows at the end, so the branch is visible from the
 * first page on.
 */
export function stableLine<T extends Lineable>(list: readonly T[], limit: number): LineItem<T>[] {
  const items: LineItem<T>[] = [];
  let segment: T[] = [];
  const flush = () => {
    for (const era of groupEras(segment)) items.push({ kind: "era", era });
    segment = [];
  };
  let shown = 0;
  let index = 0;
  while (index < list.length) {
    const release = list[index]!;
    if (release.pre) {
      const { run, end } = betaRunAt(list, index);
      flush();
      items.push({ kind: "betas", run });
      index = end + 1;
    } else if (shown < limit) {
      segment.push(release);
      shown += 1;
      index += 1;
    } else {
      const last = segment[segment.length - 1];
      const minor = minorOf(release.version);
      let next = index;
      while (next < list.length && !list[next]!.pre && minorOf(list[next]!.version) === minor) {
        next += 1;
      }
      flush();
      // Only a beta run folds here: when the next release is another stable, nothing is hidden.
      if (last && minorOf(last.version) === minor && next < list.length && list[next]!.pre) {
        items.push({ kind: "betas", run: betaRunAt(list, next).run });
      }
      return items;
    }
  }
  flush();
  return items;
}

/** How many releases one page of the list shows. */
export const CHANGELOG_PAGE = 12;

/** The list's view as the URL carries it: `?betas=1`, `?q=` and `?page=` (pages loaded). */
export interface ListState {
  betas: boolean;
  query: string;
  pages: number;
}

export function listState(params: Pick<URLSearchParams, "get">): ListState {
  const pages = Number.parseInt(params.get("page") ?? "", 10);
  return {
    betas: params.get("betas") === "1",
    query: params.get("q") ?? "",
    pages: Number.isInteger(pages) && pages > 1 ? pages : 1,
  };
}

/**
 * Where a release sits on the list: the channel that shows it and enough pages to reach it, with
 * its entry as the anchor. The release page links back there.
 */
export function listHref(line: readonly Pick<Lineable, "version" | "pre">[], version: string) {
  const current = line.find(release => release.version === version);
  if (!current) return "/changelog";
  const channel = current.pre ? line : line.filter(release => !release.pre);
  const pages = Math.ceil((channel.indexOf(current) + 1) / CHANGELOG_PAGE);
  const params = new URLSearchParams();
  if (current.pre) params.set("betas", "1");
  if (pages > 1) params.set("page", String(pages));
  const query = params.toString();
  return `/changelog${query ? `?${query}` : ""}#${encodeURIComponent(version)}`;
}
