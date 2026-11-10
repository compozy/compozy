"use client";

/** One continuous release line. Betas run as a side branch that forks before 0.4 and merges back at 0.4. */
import { Button, cn } from "@compozy/ui";
import { GitBranch } from "@compozy/ui/icons";
import { m, useInView, useScroll } from "motion/react";
import Link from "next/link";
import { useRef, type RefObject } from "react";

import { useReducedMotionSafe } from "@/components/site/hooks/use-reduced-motion-safe";
import { useScrollTransform } from "@/components/site/hooks/use-scroll-transform";
import { BEAM } from "@/components/site/layout";
import { Muted, ProductWindow, TextLink } from "@/components/site/primitives";
import { releaseCapture } from "@/lib/changelog/captures";
import {
  formatDate,
  groupEras,
  isMajor,
  type Era,
  type TimelineRelease,
} from "@/lib/changelog/timeline";

/** The beta era that branches off the main line, and the stable era it merges back into. */
const BETA_ERA = "0.3";
const MERGE_ERA = "0.4";

export interface BetaBranch {
  count: number;
  range: string;
}

function hrefOf(version: string) {
  return `/changelog/${encodeURIComponent(version)}`;
}

function Spine({ target }: { target: RefObject<HTMLDivElement | null> }) {
  const reduced = useReducedMotionSafe();
  const { scrollYProgress } = useScroll({ target, offset: ["start 55%", "end 55%"] });
  const fill = useScrollTransform(scrollYProgress, [0, 1], [0, 1]);
  const head = useScrollTransform(scrollYProgress, [0, 1], ["0%", "100%"]);
  return (
    <div
      aria-hidden
      className="pointer-events-none absolute inset-y-0 left-[var(--rail)] w-0.5 -translate-x-1/2 bg-site-light-120"
    >
      <m.span
        style={{ scaleY: reduced ? 1 : fill }}
        className={cn("absolute inset-0 origin-top", BEAM)}
      />
      {reduced ? null : (
        <m.span
          style={{ top: head }}
          className="absolute left-1/2 size-3 -translate-1/2 rounded-full bg-[#1f87fc] shadow-[0_0_24px_6px_rgba(31,135,252,0.75)]"
        />
      )}
    </div>
  );
}

function Node({ release, className }: { release: TimelineRelease; className: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  const passed = useInView(ref, { margin: "0px 0px -45% 0px" });
  const lit = useReducedMotionSafe() || passed;
  const size = isMajor(release)
    ? "size-[18px]"
    : release.lead || release.noted
      ? "size-3.5"
      : "size-2.5";
  const litTone = release.breaking
    ? "bg-warning shadow-[0_0_18px_2px_rgba(245,165,36,0.55)]"
    : release.pre
      ? "bg-fg/70"
      : "bg-[image:var(--gradient-brand)] shadow-[0_0_18px_2px_rgba(29,240,215,0.45)]";
  return (
    <span
      ref={ref}
      aria-hidden
      className={cn(
        "absolute z-10 -translate-x-1/2 rounded-full border-2 border-[#0a0a0a] transition-[background-color,box-shadow,transform] duration-500 group-hover/entry:scale-125",
        size,
        lit ? litTone : "bg-[#3a3a3a]",
        className
      )}
    />
  );
}

function Flags({ release }: { release: TimelineRelease }) {
  if (!release.pre && !release.breaking) return null;
  return (
    <p className="m-0 mb-2 flex flex-wrap gap-x-3 text-[13.5px]">
      {release.pre ? <span className="text-subtle">Beta</span> : null}
      {release.breaking ? <span className="text-warning">Breaking changes</span> : null}
    </p>
  );
}

/** The full version, beside its point on the line. */
function VersionLabel({ release, prominent }: { release: TimelineRelease; prominent: boolean }) {
  return (
    <div
      className={cn(
        "hidden w-[156px] lg:absolute lg:left-[-266px] lg:block",
        prominent ? "lg:top-[38px]" : "lg:top-[12px]"
      )}
    >
      <p
        className={cn(
          "m-0 tracking-[-0.01em] text-fg",
          prominent
            ? release.pre
              ? "font-display text-[20px] leading-none whitespace-nowrap"
              : "font-display text-[30px] leading-none"
            : "font-mono text-[14px] whitespace-nowrap"
        )}
      >
        {release.version}
      </p>
      <p
        className={cn("m-0 text-subtle", prominent ? "mt-2 text-[13.5px]" : "mt-0.5 text-[12.5px]")}
      >
        {formatDate(release.date)}
      </p>
    </div>
  );
}

function MobileMeta({ release }: { release: TimelineRelease }) {
  return (
    <p className="m-0 mb-2 flex flex-wrap items-center gap-x-3 text-[14px] lg:hidden">
      <span className="font-mono text-fg">{release.version}</span>
      <span className="text-subtle">{formatDate(release.date)}</span>
    </p>
  );
}

const NODE_MAIN = "left-[-26px] lg:left-[-48px]";
const NODE_BRANCH = "left-[calc(-26px+var(--branch))] lg:left-[calc(-48px+var(--branch))]";

function Entry({ release, branch }: { release: TimelineRelease; branch: boolean }) {
  const capture = releaseCapture(release.version);
  const rich = Boolean(release.lead || capture) || release.noted;
  const href = hrefOf(release.version);
  if (!rich) {
    return (
      <li id={release.version} className="group/entry relative scroll-mt-28 py-3">
        <Node release={release} className={cn("top-[20px]", branch ? NODE_BRANCH : NODE_MAIN)} />
        <VersionLabel release={release} prominent={false} />
        <Link
          href={href}
          className="flex w-full cursor-pointer flex-wrap items-baseline gap-x-4 gap-y-1 rounded-[12px] bg-transparent px-4 py-1.5 text-left outline-none transition-colors hover:bg-site-light-040 focus-visible:ring-2 focus-visible:ring-brand"
        >
          <span className="font-mono text-[13.5px] text-fg/80 lg:hidden">{release.version}</span>
          <span className="min-w-0 flex-1 text-[15.5px] text-muted-foreground">
            {release.title}
          </span>
          <span className="text-[13px] text-subtle lg:hidden">{formatDate(release.date)}</span>
        </Link>
      </li>
    );
  }
  return (
    <li id={release.version} className="group/entry relative scroll-mt-28 py-4">
      <Node release={release} className={cn("top-[46px]", branch ? NODE_BRANCH : NODE_MAIN)} />
      <VersionLabel release={release} prominent />
      <span
        aria-hidden
        className={cn(
          "absolute top-[53px] h-px bg-site-light-200",
          branch ? "left-0 w-0" : "left-[-26px] w-[26px] lg:left-[-48px] lg:w-[48px]"
        )}
      />
      <article className="relative overflow-hidden rounded-[22px] border border-site-light-090 bg-site-light-025 p-6 transition-[border-color,background-color] duration-300 group-hover/entry:border-site-light-200 group-hover/entry:bg-site-light-040 md:p-8">
        <MobileMeta release={release} />
        <Flags release={release} />
        <Link
          href={href}
          className="mt-3 block cursor-pointer bg-transparent p-0 text-left outline-none focus-visible:underline"
        >
          <span className="block font-display text-[26px]/[1.12] tracking-[-0.02em] text-balance text-fg md:text-[36px]/[1.08]">
            {release.title}
          </span>
        </Link>
        {capture ? (
          <ProductWindow glow={false} className="mt-6 bg-site-light-050">
            <img
              src={capture.src}
              alt={capture.alt}
              draggable={false}
              loading="lazy"
              className="block size-full object-cover object-top"
            />
          </ProductWindow>
        ) : null}
        {release.lead ? (
          <p className="m-0 mt-5 line-clamp-3 max-w-[68ch] text-[16.5px]/[1.6] text-pretty text-fg/80">
            {release.lead}
          </p>
        ) : null}
        {release.also.length > 0 ? (
          <ul className="m-0 mt-5 flex max-w-[68ch] list-none flex-col gap-2 p-0">
            {release.also.map(item => (
              <li key={item} className="flex gap-3 text-[15px]/[1.5] text-muted-foreground">
                <span
                  aria-hidden
                  className="mt-[0.6em] size-1 shrink-0 rounded-full bg-site-light-350"
                />
                {item}
              </li>
            ))}
          </ul>
        ) : null}
        <div className="mt-6 flex flex-wrap items-center justify-between gap-x-6 gap-y-2">
          {release.counts.length > 0 ? (
            <Muted className="text-[14px]">{release.counts.join(" · ")}</Muted>
          ) : (
            <span />
          )}
          <TextLink href={href} className="text-[14px]">
            Read the full notes
          </TextLink>
        </div>
      </article>
    </li>
  );
}

const GRID = "relative grid grid-cols-[44px_minmax(0,1fr)] lg:grid-cols-[170px_96px_minmax(0,1fr)]";

function BranchBracket({ className }: { className?: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        "pointer-events-none absolute left-[var(--rail)] w-[var(--branch)] rounded-r-[16px] border-y-2 border-r-2 border-site-light-250",
        className
      )}
    />
  );
}

function EraSection({ era }: { era: Era<TimelineRelease> }) {
  return (
    <section aria-label={`Version ${era.key}`} className={cn(GRID, "py-2")}>
      {era.beta ? <BranchBracket className="inset-y-8" /> : null}
      <ol className="col-start-2 m-0 list-none p-0 lg:col-start-3">
        {era.releases.map(release => (
          <Entry key={release.version} release={release} branch={era.beta} />
        ))}
      </ol>
    </section>
  );
}

function BetaLoop({ betas, onShow }: { betas: BetaBranch; onShow: () => void }) {
  return (
    <section aria-label="Beta releases" className={cn(GRID, "py-8")}>
      <BranchBracket className="inset-y-8" />
      <div className="col-start-2 mb-2 pt-8 pl-3 lg:col-start-1 lg:row-start-1 lg:mb-0 lg:pt-6 lg:pl-0">
        <p className="m-0 font-display text-[30px] leading-none tracking-[-0.01em] text-fg/60">
          {BETA_ERA} betas
        </p>
        <p className="m-0 mt-2 text-[13.5px] text-subtle">{betas.range}</p>
      </div>
      <div className="col-start-2 flex min-h-[132px] flex-wrap items-center justify-between gap-4 pl-6 lg:col-start-3 lg:pl-8">
        <div>
          <p className="m-0 inline-flex items-center gap-2.5 text-[17px] font-medium text-fg">
            <GitBranch aria-hidden className="size-4 text-subtle" />
            {betas.count} betas led to {MERGE_ERA}.0
          </p>
          <Muted className="mt-1.5">
            They branched off after 0.2.15 and merged back when CompozyOS went stable.
          </Muted>
        </div>
        <Button variant="outline" onClick={onShow}>
          Show betas
        </Button>
      </div>
    </section>
  );
}

export function ReleaseTimeline({
  releases,
  showBetas,
  betas,
  onShowBetas,
}: {
  releases: readonly TimelineRelease[];
  showBetas: boolean;
  betas: BetaBranch;
  onShowBetas: () => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  const eras = groupEras(releases);
  const keys = eras.map(era => era.key);
  const loopAt =
    !showBetas && betas.count > 0 && keys.includes(MERGE_ERA) ? keys.indexOf(MERGE_ERA) + 1 : -1;
  return (
    <div
      ref={root}
      className="relative [--branch:12px] [--rail:18px] lg:[--branch:22px] lg:[--rail:218px]"
    >
      <Spine target={root} />
      {eras.map((era, index) => (
        <div key={`${era.key}-${era.releases[0]?.version}`}>
          {index === loopAt ? <BetaLoop betas={betas} onShow={onShowBetas} /> : null}
          <EraSection era={era} />
        </div>
      ))}
      {loopAt === eras.length ? <BetaLoop betas={betas} onShow={onShowBetas} /> : null}
    </div>
  );
}
