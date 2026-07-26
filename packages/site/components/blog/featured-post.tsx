import type { Post } from "#site/content";
import { ArrowUpRight, Clock } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { AuthorMeta } from "./author-meta";
import { DateStamp } from "./date-stamp";
import { BulletDivider } from "./divider";
import { categoryLabel, formatReadingTime } from "./format";
import { BlogKindChip, type WireKind } from "./kind-chip";
import { MonoBadge } from "./mono-badge";
import { blogPostCover } from "@/lib/blog";
import { Eyebrow } from "@agh/ui";

export interface FeaturedPostProps {
  post: Post;
  authorInitial: string;
}

export function FeaturedPost({ post, authorInitial }: FeaturedPostProps) {
  const readingTime = formatReadingTime(post.metadata.readingTime);
  const cover = resolveFeaturedCover(post);

  return (
    <article className="grid gap-10 rounded-xl border border-line bg-canvas-soft p-7 lg:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)] lg:items-center">
      <div className="order-2 lg:order-1">
        <div className="flex flex-wrap items-center gap-3">
          <MonoBadge tone="accent">FEATURED</MonoBadge>
          <BulletDivider />
          <Eyebrow className="text-muted">{categoryLabel(post.category)}</Eyebrow>
          <BulletDivider />
          <DateStamp date={post.date} />
        </div>
        <h2 className="mt-6 max-w-[20ch] font-display text-site-feature-title font-normal leading-none tracking-tight text-fg">
          <Link href={post.permalink} className="transition-colors hover:text-accent">
            {post.title}
          </Link>
        </h2>
        <p className="mt-5 max-w-[54ch] text-base leading-7 text-muted">{post.description}</p>
        {post.kinds.length > 0 && (
          <div className="mt-7 flex flex-wrap items-center gap-2">
            {post.kinds.map(kind => (
              <BlogKindChip key={kind} kind={kind as WireKind} />
            ))}
          </div>
        )}
        <div className="mt-7 flex flex-wrap items-center gap-4">
          <AuthorMeta handle={post.author} initial={authorInitial} size="sm" />
          <BulletDivider />
          <span className="inline-flex items-center gap-1.5 text-xs text-subtle">
            <Clock size={12} aria-hidden />
            <span>{readingTime} read</span>
          </span>
          <Link
            href={post.permalink}
            className="ml-auto inline-flex items-center gap-1.5 text-sm font-medium text-accent"
          >
            Read post <ArrowUpRight size={14} aria-hidden />
          </Link>
        </div>
      </div>
      <div className="order-1 lg:order-2">
        {cover ? (
          <FeaturedCover
            src={cover.src}
            alt={cover.alt}
            width={cover.width}
            height={cover.height}
          />
        ) : (
          <FeaturedVisual kinds={post.kinds.length > 0 ? (post.kinds as WireKind[]) : undefined} />
        )}
      </div>
    </article>
  );
}

function resolveFeaturedCover(
  post: Post
): { src: string; alt: string; width: number; height: number } | null {
  return blogPostCover(post);
}

interface FeaturedCoverProps {
  src: string;
  alt: string;
  width: number;
  height: number;
}

function FeaturedCover({ src, alt, width, height }: FeaturedCoverProps) {
  return (
    <div className="overflow-hidden rounded-xl border border-line bg-rail">
      <Image
        src={src}
        alt={alt}
        width={width}
        height={height}
        priority
        sizes="(min-width: 1024px) 38vw, 100vw"
        className="block h-auto w-full"
      />
    </div>
  );
}

interface FeaturedVisualProps {
  kinds?: WireKind[];
}

function FeaturedVisual({ kinds }: FeaturedVisualProps) {
  const wire = (
    kinds && kinds.length >= 4 ? kinds.slice(0, 4) : ["greet", "direct", "receipt", "trace"]
  ) as WireKind[];
  const trace: { kind: WireKind; from: string; to: string; t: string }[] = [
    { kind: wire[0], from: "alpha", to: "bravo", t: "00:00.041" },
    { kind: wire[1], from: "alpha", to: "bravo", t: "00:00.108" },
    { kind: wire[2], from: "bravo", to: "alpha", t: "00:00.382" },
    { kind: wire[3], from: "bravo", to: "*", t: "00:00.384" },
  ];
  return (
    <div className="relative min-h-85 rounded-xl border border-line bg-rail p-6">
      <div className="flex items-center justify-between">
        <Eyebrow className="text-muted tracking-badge!">agh-network/v0</Eyebrow>
        <span className="inline-flex items-center gap-1.5">
          <span className="inline-block size-1.5 rounded-full bg-accent" />
          <Eyebrow className="text-accent tracking-badge!">ALPHA</Eyebrow>
        </span>
      </div>
      <div className="mt-10 grid grid-cols-3 gap-4">
        {[
          { id: "agent.alpha", role: "planner", highlight: false },
          { id: "agent.bravo", role: "executor", highlight: true },
          { id: "agent.charlie", role: "guardian", highlight: false },
        ].map(node => (
          <div
            key={node.id}
            className={`rounded-lg border bg-canvas-soft px-3 py-2.5 ${
              node.highlight ? "border-accent" : "border-line"
            }`}
          >
            <p className={`font-mono text-eyebrow ${node.highlight ? "text-accent" : "text-fg"}`}>
              {node.id}
            </p>
            <Eyebrow className="mt-1 text-subtle">{node.role}</Eyebrow>
          </div>
        ))}
      </div>
      <div className="mt-6 rounded-lg border border-line bg-canvas-soft px-3 py-2.5">
        <div className="flex items-center justify-between border-b border-line pb-2">
          <Eyebrow className="text-muted tracking-badge!">WIRE TRACE</Eyebrow>
          <Eyebrow className="text-muted">{trace.length} events</Eyebrow>
        </div>
        <ul className="mt-3 flex flex-col gap-2">
          {trace.map(row => (
            <li key={`${row.t}-${row.kind}`} className="flex items-center gap-3">
              <span className="w-16 font-mono text-badge text-subtle">{row.t}</span>
              <BlogKindChip kind={row.kind} />
              <span className="font-mono text-eyebrow text-muted">
                {row.from} <span className="text-accent">→</span> {row.to}
              </span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
