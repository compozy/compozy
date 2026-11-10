"use client";

/** Every release on one line. Taller bars shipped more changes. */
import { cn } from "@compozy/ui";
import Link from "next/link";
import { Fragment, useState } from "react";

import { formatDate, pulseSlots, type TimelineRelease } from "@/lib/changelog/timeline";

/** Half the widest hit area a bar takes, in px; neighbours closer than this share the gap. */
const HIT_HALF = 6;

function heightOf(release: TimelineRelease) {
  return Math.min(100, 14 + Math.sqrt(release.total) * 10);
}

function sinceLabel(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  });
}

export function ReleasePulse({
  releases,
  showBetas,
}: {
  releases: readonly TimelineRelease[];
  showBetas: boolean;
}) {
  const [hovered, setHovered] = useState<TimelineRelease | null>(null);
  const newest = releases[0];
  if (!newest) return null;
  const times = releases.map(release => Date.parse(release.date));
  const first = Math.min(...times);
  const last = Math.max(...times);
  const position = (time: number) =>
    last === first ? 50 : ((time - first) / (last - first)) * 100;
  const slots = pulseSlots(times);
  const months = Array.from(new Set(releases.map(release => release.date.slice(0, 7))))
    .toSorted()
    .flatMap(month => {
      const at = Date.parse(`${month}-01T00:00:00Z`);
      return at >= first - 864e5 ? [{ month, at }] : [];
    });
  const oldest = releases[releases.length - 1] ?? newest;
  const focus = hovered ?? newest;
  return (
    <div>
      {/* Wraps instead of clipping the focused release on narrow screens. */}
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 font-mono text-[13px]">
        <span className="text-muted-foreground">
          {releases.length} releases since {sinceLabel(oldest.date)}
        </span>
        <span className="whitespace-nowrap text-fg">
          {focus.version} <span className="text-subtle">· {formatDate(focus.date)}</span>
        </span>
      </div>
      <div className="relative mt-5 h-[104px]" onMouseLeave={() => setHovered(null)}>
        <span aria-hidden className="absolute inset-x-0 bottom-0 h-px bg-site-light-100" />
        {releases.map((release, index) => {
          const muted = release.pre && !showBetas;
          const slot = slots[index];
          if (!slot) return null;
          const left = `calc(${slot.at}% - ${HIT_HALF}px)`;
          const right = `calc(${100 - slot.at}% - ${HIT_HALF}px)`;
          return (
            <Fragment key={release.version}>
              <Link
                href={`/changelog/${encodeURIComponent(release.version)}`}
                aria-label={`${release.version}, ${formatDate(release.date)}`}
                onMouseEnter={() => setHovered(release)}
                onFocus={() => setHovered(release)}
                className="peer/bar absolute inset-y-0 cursor-pointer bg-transparent p-0 outline-none"
                style={{
                  left: slot.from === null ? left : `max(${left}, ${slot.from}%)`,
                  right: slot.to === null ? right : `max(${right}, ${100 - slot.to}%)`,
                }}
              />
              <span
                aria-hidden
                className={cn(
                  "pointer-events-none absolute bottom-0 block w-[3px] -translate-x-1/2 rounded-t-full transition-opacity peer-hover/bar:opacity-100 peer-focus-visible/bar:opacity-100",
                  release.pre ? "bg-site-light-350" : "bg-[image:var(--gradient-brand)]",
                  muted ? "opacity-40" : "opacity-90"
                )}
                style={{ left: `${slot.at}%`, height: `${heightOf(release)}%` }}
              />
            </Fragment>
          );
        })}
      </div>
      <div aria-hidden className="relative mt-2 h-5 font-mono text-[12px] text-subtle">
        {months.map(item => (
          <span
            key={item.month}
            className="absolute"
            style={{ left: `${Math.max(0, position(item.at))}%` }}
          >
            {new Date(item.at).toLocaleDateString("en-US", { month: "short", timeZone: "UTC" })}
          </span>
        ))}
      </div>
    </div>
  );
}
