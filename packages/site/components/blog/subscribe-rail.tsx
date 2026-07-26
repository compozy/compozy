import { ArrowUpRight, Rss } from "lucide-react";
import Link from "next/link";
import { Eyebrow } from "@agh/ui";

export function SubscribeRail() {
  return (
    <aside
      aria-label="Blog subscription links"
      className="rounded-xl border border-line bg-rail p-5"
    >
      <Eyebrow className="text-accent tracking-badge!">Stay current</Eyebrow>
      <h4 className="mt-3 font-sans text-lg font-medium leading-tight tracking-tight text-fg">
        Follow releases on the wire.
      </h4>
      <p className="mt-3 text-small-body leading-6 text-muted">
        Protocol changes, runtime drops, and the occasional engineering note. Pick your channel.
      </p>
      <div className="mt-5 flex flex-col gap-2">
        <Link
          href="/blog/feed.xml"
          className="inline-flex items-center justify-between rounded-lg border border-line bg-elevated px-3 py-2.5 text-sm font-medium text-fg transition-colors hover:bg-hover"
        >
          <span className="inline-flex items-center gap-2">
            <Rss size={14} aria-hidden />
            <span>RSS feed</span>
          </span>
          <ArrowUpRight size={14} aria-hidden className="text-subtle" />
        </Link>
        <Link
          href="/changelog"
          aria-label="Read the changelog"
          className="inline-flex items-center justify-between rounded-lg border border-line bg-elevated px-3 py-2.5 text-sm font-medium text-fg transition-colors hover:bg-hover"
        >
          <span className="inline-flex items-center gap-2">
            <ArrowUpRight size={14} aria-hidden />
            <span>Read the changelog</span>
          </span>
          <ArrowUpRight size={14} aria-hidden className="text-subtle" />
        </Link>
      </div>
      <Eyebrow className="mt-5 text-subtle">/blog/feed.xml</Eyebrow>
    </aside>
  );
}
