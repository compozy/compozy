/** One release in full: what people need to know first, every merged change last. */
import { Button, Markdown, cn } from "@compozy/ui";
import { ArrowLeft, ArrowRight, ArrowUpRight, ChevronDown } from "@compozy/ui/icons";
import { useState } from "react";

import { CONTAINER, HAIRLINE } from "../landing/layout";
import { H1, H3, Muted, ProductWindow, Small } from "../landing/primitives";
import { CAPTURES } from "./captures";
import rawContributors from "./contributors.json";
import { ReleaseMeta } from "./entry";
import {
  COMMIT_TITLES,
  GROUP_ORDER,
  REPO,
  RELEASES,
  changeCounts,
  title,
  type CommitLine,
  type Release,
  type ReleaseNote,
} from "./releases";

const CONTRIBUTORS = rawContributors as Record<string, Array<{ login: string; avatar: string }>>;

function slug(group: string) {
  return group.toLowerCase().replace(/\s+/g, "-");
}

function Note({ note }: { note: ReleaseNote }) {
  return (
    <div className="mt-9 first:mt-0">
      {note.title ? (
        <h3 className="m-0 text-[20px]/[1.3] font-medium text-fg">{note.title}</h3>
      ) : null}
      <div className="mt-3 max-w-[72ch] text-[16px] text-fg/80">
        <Markdown>{note.body}</Markdown>
      </div>
      {note.migration ? (
        <div className="mt-5 max-w-[72ch] rounded-[14px] border border-[color:var(--light-100)] bg-[color:var(--light-025)] p-5">
          <p className="m-0 mb-2 text-[14px] font-medium text-warning">Migration</p>
          <Markdown>{note.migration}</Markdown>
        </div>
      ) : null}
    </div>
  );
}

function CommitGroup({ name, lines }: { name: string; lines: CommitLine[] }) {
  return (
    <div>
      <p className="m-0 text-[15px] font-medium text-fg">{COMMIT_TITLES[name] ?? name}</p>
      <ul className="m-0 mt-3 flex list-none flex-col gap-2.5 p-0">
        {lines.map(line => (
          <li key={line.text} className="text-[14.5px]/[1.5] text-muted-foreground">
            {line.text}
            {line.prs.map(pr => (
              <a
                key={pr}
                href={`${REPO}/pull/${pr}`}
                className="ml-2 font-mono text-[12.5px] text-subtle underline-offset-4 hover:text-fg hover:underline"
              >
                #{pr}
              </a>
            ))}
          </li>
        ))}
      </ul>
    </div>
  );
}

function AllChanges({ release, startOpen }: { release: Release; startOpen: boolean }) {
  const [open, setOpen] = useState(startOpen);
  const total = Object.values(release.commits).flat().length;
  if (total === 0) return null;
  return (
    <section id="all-changes" className={cn("mt-14 scroll-mt-28 border-t pt-10", HAIRLINE)}>
      <Button variant="outline" aria-expanded={open} onClick={() => setOpen(value => !value)}>
        Every merged change · {total}
        <ChevronDown className={cn("transition-transform", open && "rotate-180")} />
      </Button>
      {open ? (
        <div className="mt-8 flex flex-col gap-9">
          {Object.entries(release.commits).map(([name, lines]) => (
            <CommitGroup key={name} name={name} lines={lines} />
          ))}
        </div>
      ) : null}
    </section>
  );
}

function neighbours(release: Release) {
  const stable = RELEASES.filter(item => item.pre === release.pre);
  const index = stable.findIndex(item => item.version === release.version);
  return { newer: stable[index - 1], older: stable[index + 1] };
}

function Trail({ release, onOpen }: { release: Release; onOpen: (version: string) => void }) {
  const same = RELEASES.filter(item => item.pre === release.pre);
  const index = same.findIndex(item => item.version === release.version);
  const near = same.slice(Math.max(0, index - 2), index + 3);
  return (
    <ol aria-label="Nearby releases" className="relative m-0 mb-8 list-none p-0">
      <span
        aria-hidden
        className="absolute top-2 bottom-2 left-[5px] w-0.5 bg-[color:var(--light-120)]"
      />
      {near.map(item => {
        const current = item.version === release.version;
        return (
          <li key={item.version} className="relative py-1.5 pl-7">
            <span
              aria-hidden
              className={cn(
                "absolute top-[13px] left-0 size-3 rounded-full border-2 border-[#0a0a0a]",
                current
                  ? "bg-[image:var(--gradient-brand)] shadow-[0_0_14px_2px_rgba(29,240,215,0.5)]"
                  : "bg-[#3a3a3a]"
              )}
            />
            <button
              type="button"
              aria-current={current ? "true" : undefined}
              onClick={() => onOpen(item.version)}
              className={cn(
                "cursor-pointer bg-transparent p-0 font-mono text-[13.5px] outline-none transition-colors hover:text-fg focus-visible:text-fg",
                current ? "text-fg" : "text-muted-foreground"
              )}
            >
              {item.version}
            </button>
          </li>
        );
      })}
    </ol>
  );
}

function Rail({
  release,
  groups,
  onOpen,
}: {
  release: Release;
  groups: string[];
  onOpen: (version: string) => void;
}) {
  const contributors = CONTRIBUTORS[release.version] ?? [];
  return (
    <aside className="lg:sticky lg:top-28 lg:self-start">
      <Trail release={release} onOpen={onOpen} />
      {groups.length > 0 ? (
        <nav aria-label="In this release" className="hidden flex-col gap-2.5 lg:flex">
          {groups.map(group => (
            <a
              key={group}
              href={`#${slug(group)}`}
              className="text-[14px] text-muted-foreground transition-colors hover:text-fg"
            >
              {group}
            </a>
          ))}
          <a
            href="#all-changes"
            className="text-[14px] text-muted-foreground transition-colors hover:text-fg"
          >
            Every merged change
          </a>
        </nav>
      ) : null}
      {contributors.length > 0 ? (
        <div className={cn(groups.length > 0 && "lg:mt-8 lg:border-t lg:pt-6", HAIRLINE)}>
          <Small>Contributors</Small>
          <div className="mt-3 flex flex-wrap gap-2">
            {contributors.map(person => (
              <a
                key={person.login}
                href={`https://github.com/${person.login}`}
                className="inline-flex items-center gap-2 text-[14px] text-fg"
              >
                <img src={person.avatar} alt="" className="size-6 rounded-full" />
                {person.login}
              </a>
            ))}
          </div>
        </div>
      ) : null}
      <a
        href={release.url}
        className="mt-6 inline-flex items-center gap-1 text-[14px] text-muted-foreground transition-colors hover:text-fg"
      >
        Release files on GitHub <ArrowUpRight aria-hidden className="size-3.5" />
      </a>
    </aside>
  );
}

export function ReleaseDetail({
  release,
  onBack,
  onOpen,
}: {
  release: Release;
  onBack: () => void;
  onOpen: (version: string) => void;
}) {
  const capture = CAPTURES[release.version];
  const groups = GROUP_ORDER.filter(group => release.notes.some(note => note.group === group));
  const counts = changeCounts(release);
  const { newer, older } = neighbours(release);
  return (
    <div className={cn(CONTAINER, "pt-[120px] pb-24 md:pt-[136px]")}>
      <button
        type="button"
        onClick={onBack}
        className="inline-flex cursor-pointer items-center gap-2 bg-transparent p-0 text-[14px] text-muted-foreground outline-none transition-colors hover:text-fg focus-visible:text-fg"
      >
        <ArrowLeft aria-hidden className="size-4" /> Changelog
      </button>
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
          {capture ? (
            <ProductWindow glow={false} className="mt-9 bg-[color:var(--light-050)]">
              <img
                src={capture.src}
                alt={capture.alt}
                draggable={false}
                className="block size-full object-cover object-top"
              />
            </ProductWindow>
          ) : null}
          {groups.map(group => (
            <section key={group} id={slug(group)} className="mt-14 scroll-mt-28">
              <H3 className={group === "Breaking Changes" ? "text-warning" : undefined}>{group}</H3>
              <div className="mt-6">
                {release.notes.flatMap(note =>
                  note.group === group
                    ? [
                        <Note
                          key={`${note.group}-${note.title ?? note.body.slice(0, 24)}`}
                          note={note}
                        />,
                      ]
                    : []
                )}
              </div>
            </section>
          ))}
          {release.extras.map(extra => (
            <section key={extra.heading} className="mt-14">
              <H3>{extra.heading}</H3>
              <div className="mt-5 max-w-[72ch] text-[16px] text-fg/80">
                <Markdown>{extra.body}</Markdown>
              </div>
            </section>
          ))}
          <AllChanges release={release} startOpen={groups.length === 0} />
          <nav
            aria-label="Other releases"
            className={cn("mt-16 flex items-center justify-between gap-4 border-t pt-8", HAIRLINE)}
          >
            {older ? (
              <button
                type="button"
                onClick={() => onOpen(older.version)}
                className="inline-flex cursor-pointer items-center gap-2 bg-transparent p-0 font-mono text-[14px] text-muted-foreground outline-none hover:text-fg"
              >
                <ArrowLeft aria-hidden className="size-4" /> {older.version}
              </button>
            ) : (
              <span />
            )}
            {newer ? (
              <button
                type="button"
                onClick={() => onOpen(newer.version)}
                className="inline-flex cursor-pointer items-center gap-2 bg-transparent p-0 font-mono text-[14px] text-muted-foreground outline-none hover:text-fg"
              >
                {newer.version} <ArrowRight aria-hidden className="size-4" />
              </button>
            ) : null}
          </nav>
        </div>
        <Rail release={release} groups={groups} onOpen={onOpen} />
      </div>
    </div>
  );
}
