import type { ChangelogRelease } from "@/lib/changelog/types";
import { ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { DateStamp } from "./date-stamp";
import { MonoBadge } from "./mono-badge";

export interface ReleaseEntryProps {
  release: ChangelogRelease;
}

export function ReleaseEntry({ release }: ReleaseEntryProps) {
  const detailPath = `/changelog/${encodeURIComponent(release.version)}`;

  return (
    <article
      id={release.version}
      className="grid scroll-mt-24 gap-8 border-t border-line py-12 lg:grid-cols-[160px_minmax(0,1fr)] lg:gap-12"
    >
      <div className="flex flex-col gap-3 lg:sticky lg:top-24 lg:self-start">
        <DateStamp date={release.publishedAt} tracking="wide" />
        <div className="flex items-center gap-2">
          <MonoBadge>{release.channel.toUpperCase()}</MonoBadge>
          <span className="font-mono text-eyebrow text-subtle">{release.assets.length} assets</span>
        </div>
      </div>
      <div>
        <h2 className="font-sans text-site-release-title font-semibold leading-tight tracking-tight text-fg">
          <Link href={detailPath} className="hover:text-accent">
            {release.version}
          </Link>
        </h2>
        <p className="mt-4 max-w-[68ch] text-base leading-7 text-muted">{release.summary}</p>
        <div className="mt-6 flex flex-wrap gap-2" aria-label="Release categories">
          {release.categories.map(category => (
            <MonoBadge key={category.id}>
              {category.label} · {category.itemCount}
            </MonoBadge>
          ))}
        </div>
        <div className="mt-7 flex flex-wrap items-center gap-5 text-sm font-medium">
          <Link href={detailPath} className="text-accent hover:text-accent-hover">
            Read full release notes →
          </Link>
          <a
            href={release.url}
            target="_blank"
            rel="noreferrer noopener"
            className="inline-flex items-center gap-1.5 text-muted hover:text-fg"
          >
            GitHub release <ArrowUpRight size={13} aria-hidden />
          </a>
        </div>
      </div>
    </article>
  );
}
