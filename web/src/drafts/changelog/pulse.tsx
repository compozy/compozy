/** Every release on one line. Taller bars shipped more changes. */
import { cn } from "@compozy/ui";
import { useState } from "react";

import { RELEASES, formatDate, totalChanges, type Release } from "./releases";

const TIMES = RELEASES.map(release => Date.parse(release.date));
const FIRST = Math.min(...TIMES);
const LAST = Math.max(...TIMES);

const MONTHS = Array.from(new Set(RELEASES.map(release => release.date.slice(0, 7))))
  .sort()
  .map(month => ({ month, at: Date.parse(`${month}-01T00:00:00Z`) }));

const VISIBLE_MONTHS = MONTHS.filter(item => item.at >= FIRST - 864e5);

function position(time: number) {
  return ((time - FIRST) / (LAST - FIRST)) * 100;
}

function heightOf(release: Release) {
  return Math.min(100, 14 + Math.sqrt(totalChanges(release)) * 10);
}

export function ReleasePulse({
  showBetas,
  onOpen,
}: {
  showBetas: boolean;
  onOpen: (version: string) => void;
}) {
  const [hovered, setHovered] = useState<Release | null>(null);
  const focus = hovered ?? RELEASES[0];
  return (
    <div>
      <div className="flex items-baseline justify-between gap-4 font-mono text-[13px]">
        <span className="text-muted-foreground">{RELEASES.length} releases since April 2026</span>
        <span className="truncate text-fg">
          {focus.version} <span className="text-subtle">· {formatDate(focus.date)}</span>
        </span>
      </div>
      <div className="relative mt-5 h-[104px]" onMouseLeave={() => setHovered(null)}>
        <span
          aria-hidden
          className="absolute inset-x-0 bottom-0 h-px bg-[color:var(--light-100)]"
        />
        {RELEASES.map(release => {
          const muted = release.pre && !showBetas;
          return (
            <button
              key={release.version}
              type="button"
              aria-label={`${release.version}, ${formatDate(release.date)}`}
              onMouseEnter={() => setHovered(release)}
              onFocus={() => setHovered(release)}
              onClick={() => onOpen(release.version)}
              className="group/bar absolute bottom-0 flex h-full w-3 -translate-x-1/2 cursor-pointer items-end justify-center bg-transparent p-0 outline-none"
              style={{ left: `${position(Date.parse(release.date))}%` }}
            >
              <span
                className={cn(
                  "block w-[3px] rounded-t-full transition-opacity group-hover/bar:opacity-100 group-focus-visible/bar:opacity-100",
                  release.pre ? "bg-[color:var(--light-350)]" : "bg-[image:var(--gradient-brand)]",
                  muted ? "opacity-40" : "opacity-90"
                )}
                style={{ height: `${heightOf(release)}%` }}
              />
            </button>
          );
        })}
      </div>
      <div aria-hidden className="relative mt-2 h-5 font-mono text-[12px] text-subtle">
        {VISIBLE_MONTHS.map(item => (
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
