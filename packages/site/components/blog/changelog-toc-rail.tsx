import type { Release } from "#site/content";
import { cn, Eyebrow } from "@agh/ui";
import Link from "next/link";

export interface ChangelogTocRailProps {
  releases: Release[];
  activeVersion?: string;
}

export function ChangelogTocRail({ releases, activeVersion }: ChangelogTocRailProps) {
  return (
    <aside aria-label="Changelog versions" className="sticky top-20 flex flex-col gap-9 self-start">
      <div>
        <Eyebrow className="text-muted tracking-badge!">All versions</Eyebrow>
        <ul className="mt-4 flex flex-col gap-2.5">
          {releases.map(release => {
            const isActive = activeVersion ? release.version === activeVersion : false;
            return (
              <li key={release.version}>
                <Link
                  href={`#${release.version}`}
                  aria-current={isActive ? "location" : undefined}
                  className={cn(
                    "block font-mono text-small-body tracking-mono",
                    isActive ? "text-accent" : "text-muted hover:text-fg"
                  )}
                >
                  {release.version}
                </Link>
              </li>
            );
          })}
        </ul>
      </div>
      <div className="rounded-xl border border-line bg-canvas-soft p-5">
        <Eyebrow className="text-accent tracking-badge!">Upgrade</Eyebrow>
        <p className="mt-3 text-small-body leading-6 text-muted">
          One binary, one daemon. Pull the latest from Go and restart the process.
        </p>
        <Link
          href="/runtime/core/getting-started/installation"
          className="mt-4 inline-flex items-center gap-1.5 text-xs font-medium text-accent"
        >
          Install instructions →
        </Link>
      </div>
    </aside>
  );
}
