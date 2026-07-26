import type { Release } from "#site/content";
import { ArrowUpRight } from "lucide-react";
import { DateStamp } from "./date-stamp";
import { MonoBadge, type MonoBadgeTone } from "./mono-badge";

export interface ReleaseEntryProps {
  release: Release;
}

const statusTone: Record<Release["status"], MonoBadgeTone> = {
  stable: "success",
  beta: "info",
  alpha: "accent",
  breaking: "danger",
};

const sectionLabel: Record<"added" | "changed" | "fixed" | "breaking", string> = {
  added: "ADDED",
  changed: "CHANGED",
  fixed: "FIXED",
  breaking: "BREAKING",
};

const sectionTone: Record<"added" | "changed" | "fixed" | "breaking", MonoBadgeTone> = {
  added: "success",
  changed: "info",
  fixed: "warning",
  breaking: "danger",
};

function releaseBulletEntries(items: ReadonlyArray<string>) {
  const occurrenceCounts = new Map<string, number>();

  return items.map(item => {
    const occurrence = (occurrenceCounts.get(item) ?? 0) + 1;
    occurrenceCounts.set(item, occurrence);

    return {
      item,
      key: `${item}-${occurrence}`,
    };
  });
}

export function ReleaseEntry({ release }: ReleaseEntryProps) {
  const sections = (["added", "changed", "fixed", "breaking"] as const).filter(
    key => release[key].length > 0
  );

  return (
    <article
      id={release.version}
      className="grid scroll-mt-24 gap-8 border-t border-line py-12 lg:grid-cols-[160px_minmax(0,1fr)] lg:gap-12"
    >
      <div className="flex flex-col gap-3 lg:sticky lg:top-24 lg:self-start">
        <DateStamp date={release.date} tracking="wide" />
        <div className="flex items-center gap-2">
          <MonoBadge tone={statusTone[release.status]}>{release.version}</MonoBadge>
          <MonoBadge tone="neutral">{release.status.toUpperCase()}</MonoBadge>
        </div>
        {release.compareUrl && (
          <a
            href={release.compareUrl}
            target="_blank"
            rel="noreferrer noopener"
            aria-label={`Compare ${release.version} on GitHub`}
            className="inline-flex items-center gap-1.5 text-xs text-muted hover:text-fg"
          >
            Compare on GitHub <ArrowUpRight size={12} aria-hidden />
          </a>
        )}
      </div>
      <div>
        <h2 className="font-sans text-site-release-title font-semibold leading-tight tracking-tight text-fg">
          {release.summary}
        </h2>
        <div className="mt-8 flex flex-col gap-7">
          {sections.map(key => (
            <section key={key}>
              <MonoBadge tone={sectionTone[key]}>{sectionLabel[key]}</MonoBadge>
              <ul className="mt-4 flex flex-col gap-2.5">
                {releaseBulletEntries(release[key]).map(({ item, key: itemKey }) => (
                  <li
                    key={`${release.version}-${key}-${itemKey}`}
                    className="flex items-start gap-3 font-sans text-item-title leading-7 text-muted"
                  >
                    <span className="mt-2 inline-block size-1 shrink-0 rounded-sm bg-accent" />
                    <span>{item}</span>
                  </li>
                ))}
              </ul>
            </section>
          ))}
        </div>
      </div>
    </article>
  );
}
