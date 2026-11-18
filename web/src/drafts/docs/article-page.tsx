/** The standard page: masthead, a reading column, and the section rail. */
import { useRef, type ReactNode } from "react";

import { DocBlocks } from "./blocks";
import { DocMasthead, DocPager } from "./masthead";
import type { Doc, TocItem } from "./mdx";
import type { DocPage } from "./nav";
import { DocToc } from "./toc";

/** The two-column frame every reading page shares; templates pass their own body and rail items. */
export function PageFrame(props: {
  page: DocPage;
  source: string;
  facts?: string;
  toc: readonly TocItem[];
  title?: ReactNode;
  children: ReactNode;
}) {
  const body = useRef<HTMLElement>(null);
  return (
    <div className="grid gap-x-14 px-5 md:px-10 xl:grid-cols-[minmax(0,1fr)_224px] 2xl:gap-x-20">
      <article ref={body} className="min-w-0 max-w-[800px] pt-10 pb-24">
        <DocMasthead
          page={props.page}
          source={props.source}
          facts={props.facts}
          title={props.title}
        />
        <div className="pt-9">{props.children}</div>
        <DocPager slug={props.page.slug} />
      </article>
      <DocToc toc={props.toc} body={body} />
    </div>
  );
}

export function ArticlePage({ page, doc }: { page: DocPage; doc: Doc }) {
  return (
    <PageFrame page={page} source={doc.source} facts={`${doc.minutes} min read`} toc={doc.toc}>
      <DocBlocks blocks={doc.blocks} />
    </PageFrame>
  );
}

export function PageSkeleton() {
  return (
    <div aria-busy className="px-5 pt-10 pb-24 md:px-10">
      <div className="h-4 w-56 rounded-full bg-[color:var(--light-060)]" />
      <div className="mt-7 h-12 w-[min(520px,80%)] rounded-[12px] bg-[color:var(--light-060)]" />
      <div className="mt-5 h-5 w-[min(640px,90%)] rounded-full bg-[color:var(--light-040)]" />
      <div className="mt-16 flex max-w-[720px] flex-col gap-4">
        <div className="h-4 rounded-full bg-[color:var(--light-040)]" />
        <div className="h-4 w-11/12 rounded-full bg-[color:var(--light-040)]" />
        <div className="h-4 w-4/5 rounded-full bg-[color:var(--light-040)]" />
      </div>
    </div>
  );
}
