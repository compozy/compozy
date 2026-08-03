import { Eyebrow } from "@compozy/ui";
import type { ChangelogRelease } from "@/lib/changelog/types";
import { MonoBadge } from "./mono-badge";

export function ReleaseDetailRail({ release }: { release: ChangelogRelease }) {
  return (
    <aside
      aria-label="Release navigation"
      className="flex flex-col gap-8 lg:sticky lg:top-20 lg:self-start"
    >
      {release.headings.length > 0 && (
        <nav aria-label="On this release">
          <Eyebrow className="text-muted">On this release</Eyebrow>
          <ul className="mt-4 flex flex-col gap-2.5">
            {release.headings.map(heading => (
              <li
                key={heading.id}
                className={
                  heading.level === 4
                    ? "pl-3"
                    : heading.level === 5
                      ? "pl-6"
                      : heading.level === 6
                        ? "pl-9"
                        : undefined
                }
              >
                <a href={`#${heading.id}`} className="text-sm text-muted hover:text-fg">
                  {heading.label}
                </a>
              </li>
            ))}
          </ul>
        </nav>
      )}
      <div className="rounded-xl border border-line bg-canvas-soft p-5">
        <Eyebrow className="text-muted">Release record</Eyebrow>
        <div className="mt-4 flex flex-wrap gap-2">
          <MonoBadge>{release.channel.toUpperCase()}</MonoBadge>
          <MonoBadge>{release.assets.length} ASSETS</MonoBadge>
        </div>
        <a
          href={release.url}
          target="_blank"
          rel="noreferrer noopener"
          className="mt-5 inline-flex text-sm font-medium text-accent"
        >
          View release on GitHub →
        </a>
      </div>
    </aside>
  );
}
