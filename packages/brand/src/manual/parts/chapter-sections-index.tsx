// Minimal chapter body while its sections are being ported: every section keeps its anchor,
// so the in-page index, deep links and crawl targets already resolve.
import { card, lab } from "../cls";
import type { Chapter } from "../chapters";
import { StatusTag } from "./status-tag";

export function ChapterSectionsIndex({ page }: { page: Chapter }) {
  return (
    <section data-chapter-port="pending" className="band-ink py-[clamp(64px,8vw,120px)]">
      <div className="wrap">
        <div className="flex flex-wrap items-center gap-3">
          <p className={lab}>Sections · {page.sections.length}</p>
          <StatusTag status={page.status} />
        </div>
        <p className="mt-4 mb-10 max-w-[62ch] text-lead text-(--b-muted)">{page.summary}</p>
        <ol className="m-0 grid list-none gap-4 p-0 sm:grid-cols-2 lg:grid-cols-3">
          {page.sections.map(([id, label], k) => (
            <li key={id} id={id} className={`${card} scroll-mt-28`}>
              <p className="m-0 font-mono text-[12px] text-(--b-accent)">
                {String(k + 1).padStart(2, "0")}
              </p>
              <p className="m-0 mt-4 font-display text-[22px] tracking-[-0.015em]">{label}</p>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}
