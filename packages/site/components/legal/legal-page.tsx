import { cn } from "@compozy/ui";
import Link from "next/link";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

import { DOCS_PROSE } from "@/components/docs/docs-classes";
import { DocsH2, DocsInlineCode, DocsLink, DocsTable } from "@/components/docs/docs-prose";
import { CONTAINER, FOCUS, HAIRLINE } from "@/components/site/layout";
import { LEGAL_DOC_IDS, LEGAL_DOCUMENTS, type LegalDocId } from "./legal-content";
import { LegalDesktopToc, LegalMobileToc } from "./legal-toc";

const MARKDOWN_COMPONENTS: Components = {
  a: ({ node: _node, ...props }) => <DocsLink {...props} />,
  table: ({ node: _node, ...props }) => <DocsTable {...props} />,
  code: ({ node: _node, ...props }) => <DocsInlineCode {...props} />,
};

export function LegalPage({
  docId,
  mobileTocDefaultOpen = false,
}: {
  docId: LegalDocId;
  mobileTocDefaultOpen?: boolean;
}) {
  const doc = LEGAL_DOCUMENTS[docId];
  const tocItems = doc.sections.map(section => ({ id: section.id, title: section.title }));

  return (
    <div className="pt-28 pb-24 md:pt-36">
      <div
        className={cn(
          CONTAINER,
          "grid gap-x-16 xl:grid-cols-[minmax(0,760px)_240px] xl:justify-between"
        )}
      >
        <article className="min-w-0">
          <header className={cn("border-b pb-8", HAIRLINE)}>
            <div className="flex flex-wrap items-center justify-between gap-4">
              <nav
                aria-label="Breadcrumb"
                className="flex items-center gap-2 text-[13.5px] text-muted-foreground"
              >
                <span>Legal</span>
                <span aria-hidden className="text-subtle">
                  /
                </span>
                <span className="text-fg">{doc.title}</span>
              </nav>

              <nav aria-label="Legal documents" className="flex items-center gap-1">
                {LEGAL_DOC_IDS.map(tabId => {
                  const item = LEGAL_DOCUMENTS[tabId];
                  const selected = tabId === docId;
                  return (
                    <Link
                      key={tabId}
                      href={item.route}
                      aria-current={selected ? "page" : undefined}
                      className={cn(
                        "rounded-[8px] px-2.5 py-1 text-[13px] transition-colors",
                        selected
                          ? "bg-site-light-100 text-fg"
                          : "text-muted-foreground hover:text-fg",
                        FOCUS
                      )}
                    >
                      {item.navLabel}
                    </Link>
                  );
                })}
              </nav>
            </div>

            <h1 className="m-0 mt-6 font-display text-[36px]/[1.08] font-normal tracking-[-0.03em] text-balance text-fg md:text-[50px]/[1.05]">
              {doc.title}
            </h1>
            <p className="m-0 mt-4 max-w-[660px] text-[17.5px]/[1.55] text-pretty text-muted-foreground md:text-[18.5px]">
              {doc.summary}
            </p>
            <p className="m-0 mt-5 font-mono text-[12.5px] text-subtle">
              Last updated: {doc.lastUpdated}
            </p>

            <LegalMobileToc items={tocItems} defaultOpen={mobileTocDefaultOpen} />
          </header>

          <div className={cn("pt-9 [&_h2]:scroll-mt-28 [&_h3]:scroll-mt-28", DOCS_PROSE)}>
            {doc.sections.map(section => (
              <section key={section.id} aria-labelledby={section.id} className="mt-9 first:mt-0">
                <DocsH2 id={section.id}>{section.title}</DocsH2>
                <div className={DOCS_PROSE}>
                  <ReactMarkdown remarkPlugins={[remarkGfm]} components={MARKDOWN_COMPONENTS}>
                    {section.body}
                  </ReactMarkdown>
                </div>
              </section>
            ))}
          </div>
        </article>

        <LegalDesktopToc items={tocItems} />
      </div>
    </div>
  );
}
