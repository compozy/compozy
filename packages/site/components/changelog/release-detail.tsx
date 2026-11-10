/** One release in full: what people need to know first, every merged change last. */
import { Markdown, cn } from "@compozy/ui";
import { ArrowLeft, ArrowRight, ChevronDown } from "@compozy/ui/icons";
import Link from "next/link";
import type { ReactNode } from "react";

import { formatBytes } from "@/components/blog/format";
import { CONTAINER, HAIRLINE } from "@/components/site/layout";
import { H1, H2, Muted, ProductWindow } from "@/components/site/primitives";
import { releaseCapture } from "@/lib/changelog/captures";
import {
  COMMIT_TITLES,
  GROUP_ORDER,
  changeCounts,
  formatDate,
  hasBreaking,
  sectionSlug,
  title,
  totalChanges,
  type CommitLine,
  type ReleaseNote,
  type ReleaseView,
} from "@/lib/changelog/timeline";
import {
  COMPOZY_REPOSITORY,
  type ChangelogAsset,
  type ChangelogContributor,
  type ChangelogPullRequest,
} from "@/lib/changelog/types";
import { AllChanges } from "./all-changes";
import { ReleaseRail, type TrailRelease } from "./release-rail";

export interface ReleaseRecord {
  assets: readonly ChangelogAsset[];
  sourceArchiveUrl: string;
  sourceZipUrl: string;
  contributors: readonly ChangelogContributor[];
  /** Linked pull requests that no merged-change line names. */
  otherPullRequests: readonly ChangelogPullRequest[];
  comparison: { version: string; url: string } | null;
}

/** A section heading: the second level of the page, set at the draft's third-level display size. */
const SECTION = "text-[24px]/[1.15] md:text-[30px]/[1.12]";

/** The draft's prose size: the app's 13.5px body token (the site's own `--text-body` is smaller). */
const PROSE = "text-[13.5px]";

const ROW_LINK = "text-subtle underline-offset-4 hover:text-fg hover:underline";

function ReleaseMeta({ release }: { release: ReleaseView }) {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 lg:flex-col lg:items-start">
      <span className="font-mono text-[14px] text-fg">{release.version}</span>
      <span className="text-[14px] text-subtle">{formatDate(release.date)}</span>
      {release.pre ? <span className="text-[13px] text-subtle">Beta</span> : null}
      {hasBreaking(release) ? (
        <span className="text-[13px] text-warning">Breaking changes</span>
      ) : null}
    </div>
  );
}

function Note({ note }: { note: ReleaseNote }) {
  return (
    <div className="mt-9 first:mt-0">
      {note.title ? (
        <h3 className="m-0 text-[20px]/[1.3] font-medium text-fg">{note.title}</h3>
      ) : null}
      <div className="mt-3 max-w-[72ch] text-[16px] text-fg/80">
        <Markdown className={PROSE}>{note.body}</Markdown>
      </div>
      {note.migration ? (
        <div className="mt-5 max-w-[72ch] rounded-[14px] border border-site-light-100 bg-site-light-025 p-5">
          <p className="m-0 mb-2 text-[14px] font-medium text-warning">Migration</p>
          <Markdown className={PROSE}>{note.migration}</Markdown>
        </div>
      ) : null}
    </div>
  );
}

function ChangeGroup({ name, children }: { name: string; children: ReactNode }) {
  return (
    <div>
      <p className="m-0 text-[15px] font-medium text-fg">{name}</p>
      <ul className="m-0 mt-3 flex list-none flex-col gap-2.5 p-0">{children}</ul>
    </div>
  );
}

const ROW = "text-[14.5px]/[1.5] text-muted-foreground";

function CommitGroup({ name, lines }: { name: string; lines: CommitLine[] }) {
  return (
    <ChangeGroup name={COMMIT_TITLES[name] ?? name}>
      {lines.map(line => (
        <li key={`${line.text}-${line.prs.join(",")}`} className={ROW}>
          {line.text}
          {line.prs.map(pr => (
            <a
              key={pr}
              href={`${COMPOZY_REPOSITORY.url}/pull/${pr}`}
              className={cn("ml-2 font-mono text-[12.5px]", ROW_LINK)}
            >
              #{pr}
            </a>
          ))}
        </li>
      ))}
    </ChangeGroup>
  );
}

/** Every published file, folded: the full asset record without burying the change list. */
function ReleaseFiles({ record }: { record: ReleaseRecord }) {
  return (
    <details className="group/files">
      <summary className="inline-flex cursor-pointer list-none items-center gap-2 text-[15px] font-medium text-fg outline-none focus-visible:ring-2 focus-visible:ring-brand [&::-webkit-details-marker]:hidden">
        Release files · {record.assets.length + 2}
        <ChevronDown
          aria-hidden
          className="size-4 text-subtle transition-transform group-open/files:rotate-180"
        />
      </summary>
      <ul className="m-0 mt-3 flex list-none flex-col gap-2.5 p-0">
        {record.assets.map(asset => (
          <li key={asset.name} className={ROW}>
            <a href={asset.url} className="font-mono text-[13px] break-all text-fg hover:underline">
              {asset.name}
            </a>
            <span className="ml-2 text-[13px] text-subtle">
              {formatBytes(asset.size)} · {asset.downloadCount} downloads
            </span>
          </li>
        ))}
        <li className={ROW}>
          <a href={record.sourceArchiveUrl} className={ROW_LINK}>
            Source code (tar.gz)
          </a>
        </li>
        <li className={ROW}>
          <a href={record.sourceZipUrl} className={ROW_LINK}>
            Source code (zip)
          </a>
        </li>
      </ul>
    </details>
  );
}

function Neighbour({ release, newer }: { release: TrailRelease | undefined; newer: boolean }) {
  if (!release) return <span />;
  return (
    <Link
      href={`/changelog/${encodeURIComponent(release.version)}`}
      className="inline-flex cursor-pointer items-center gap-2 bg-transparent p-0 font-mono text-[14px] text-muted-foreground outline-none hover:text-fg"
    >
      {newer ? null : <ArrowLeft aria-hidden className="size-4" />} {release.version}{" "}
      {newer ? <ArrowRight aria-hidden className="size-4" /> : null}
    </Link>
  );
}

export function ReleaseDetail({
  release,
  line,
  record,
  back,
}: {
  release: ReleaseView;
  /** Every release on the same channel as this one, newest first. */
  line: readonly TrailRelease[];
  record: ReleaseRecord;
  /** The list, at this release's place in it. */
  back: string;
}) {
  const capture = releaseCapture(release.version);
  const known = new Set<string>(GROUP_ORDER);
  const groups = [
    ...GROUP_ORDER.filter(group => release.notes.some(note => note.group === group)),
    ...new Set(release.notes.flatMap(note => (known.has(note.group) ? [] : [note.group]))),
  ];
  const counts = changeCounts(release);
  const index = line.findIndex(item => item.version === release.version);
  const newer = index > 0 ? line[index - 1] : undefined;
  const older = index >= 0 ? line[index + 1] : undefined;
  return (
    <div className={cn(CONTAINER, "pt-[120px] pb-24 md:pt-[136px]")}>
      <Link
        href={back}
        className="inline-flex cursor-pointer items-center gap-2 bg-transparent p-0 text-[14px] text-muted-foreground outline-none transition-colors hover:text-fg focus-visible:text-fg"
      >
        <ArrowLeft aria-hidden className="size-4" /> Changelog
      </Link>
      <div className="mt-10 grid gap-10 lg:grid-cols-[minmax(0,1fr)_240px] lg:gap-16">
        <div className="min-w-0">
          <ReleaseMeta release={release} />
          <div
            className={cn(
              "mt-5 [&_h1]:text-[34px]",
              groups.length > 0
                ? "[&_h1]:md:text-[56px] [&_h1]:md:leading-[1.05]"
                : "[&_h1]:md:text-[40px] [&_h1]:md:leading-[1.1]"
            )}
          >
            <H1>{title(release)}</H1>
          </div>
          {counts.length > 0 ? <Muted className="mt-5">{counts.join(" · ")}</Muted> : null}
          {release.intro.map(paragraph => (
            <div key={paragraph} className="mt-6 max-w-[72ch] text-[16px] text-fg/80">
              <Markdown className={PROSE}>{paragraph}</Markdown>
            </div>
          ))}
          {capture ? (
            <ProductWindow glow={false} className="mt-9 bg-site-light-050">
              <img
                src={capture.src}
                alt={capture.alt}
                draggable={false}
                className="block size-full object-cover object-top"
              />
            </ProductWindow>
          ) : null}
          {groups.map(group => (
            <section key={group} id={sectionSlug(group)} className="mt-14 scroll-mt-28">
              <H2 className={cn(SECTION, group === "Breaking Changes" && "text-warning")}>
                {group}
              </H2>
              <div className="mt-6">
                {release.notes.flatMap((note, noteIndex) =>
                  note.group === group ? [<Note key={`${group}-${noteIndex}`} note={note} />] : []
                )}
              </div>
            </section>
          ))}
          {release.extras.map((extra, extraIndex) => (
            <section key={`${extraIndex}-${extra.heading}`} className="mt-14">
              <H2 className={SECTION}>{extra.heading}</H2>
              <div className="mt-5 max-w-[72ch] text-[16px] text-fg/80">
                <Markdown className={PROSE}>{extra.body}</Markdown>
              </div>
            </section>
          ))}
          <AllChanges total={totalChanges(release)} startOpen={groups.length === 0}>
            {Object.entries(release.commits).map(([name, lines]) => (
              <CommitGroup key={name} name={name} lines={lines} />
            ))}
            {record.otherPullRequests.length > 0 ? (
              <ChangeGroup name="Other pull requests">
                {record.otherPullRequests.map(pullRequest => (
                  <li key={pullRequest.number} className={ROW}>
                    {pullRequest.title}
                    <a
                      href={pullRequest.url}
                      className={cn("ml-2 font-mono text-[12.5px]", ROW_LINK)}
                    >
                      #{pullRequest.number}
                    </a>
                  </li>
                ))}
              </ChangeGroup>
            ) : null}
            <ReleaseFiles record={record} />
          </AllChanges>
          <nav
            aria-label="Other releases"
            className={cn("mt-16 flex items-center justify-between gap-4 border-t pt-8", HAIRLINE)}
          >
            <Neighbour release={older} newer={false} />
            {newer ? <Neighbour release={newer} newer /> : null}
          </nav>
        </div>
        <ReleaseRail
          version={release.version}
          line={line}
          groups={groups}
          contributors={record.contributors}
          url={release.url}
          comparison={record.comparison}
        />
      </div>
    </div>
  );
}
