import { cn } from "@compozy/ui";
import { ChevronRight } from "@compozy/ui/icons";
import { useLayoutEffect, useState } from "react";

import type { TocItem } from "../docs/mdx";
import { DocsProse } from "../docs/prose";
import { BEAM, FOCUS, LINE } from "../docs/text";
import { CONTAINER, HAIRLINE } from "../landing/layout";
import { SiteFooter } from "../landing/sections-close";
import { SiteHeader } from "../landing/sections-open";
import { LEGAL_DOCUMENTS, type LegalDocId } from "./legal-content";

const LEGAL_TABS: readonly LegalDocId[] = ["terms", "privacy", "refunds"];

function LegalTocList({
  toc,
  activeId,
  onSelect,
}: {
  toc: readonly TocItem[];
  activeId: string;
  onSelect: (id: string) => void;
}) {
  return (
    <ol className="m-0 list-none p-0">
      {toc.map(item => {
        const current = activeId === item.id;
        return (
          <li key={item.id}>
            <a
              href={`#${item.id}`}
              aria-current={current ? "location" : undefined}
              onClick={event => {
                event.preventDefault();
                onSelect(item.id);
                document
                  .getElementById(item.id)
                  ?.scrollIntoView({ behavior: "instant", block: "start" });
              }}
              className={cn(
                "block py-1.5 text-[13.5px]/[1.4] transition-colors",
                item.depth === 3 ? "pl-8" : "pl-4",
                current ? "text-fg" : "text-muted-foreground hover:text-fg"
              )}
            >
              {item.text}
            </a>
          </li>
        );
      })}
    </ol>
  );
}

function MobileTocDisclosure({
  toc,
  defaultOpen,
  activeId,
  onSelect,
}: {
  toc: readonly TocItem[];
  defaultOpen: boolean;
  activeId: string;
  onSelect: (id: string) => void;
}) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div className={cn("mt-7 rounded-[14px] border bg-[#0e0e0e] xl:hidden", LINE)}>
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen(value => !value)}
        className={cn(
          "flex h-11 w-full cursor-pointer items-center gap-2 rounded-[14px] px-4 text-[13.5px] text-muted-foreground transition-colors hover:text-fg",
          FOCUS
        )}
      >
        <ChevronRight
          aria-hidden
          className={cn("size-3.5 text-subtle transition-transform", open && "rotate-90")}
        />
        <span>On this page</span>
        <span className="ml-auto font-mono text-[11.5px] text-subtle tabular-nums">
          {toc.length}
        </span>
      </button>
      {open ? (
        <div className="relative mx-4 mb-4 border-l border-[color:var(--light-120)]">
          <LegalTocList
            toc={toc}
            activeId={activeId}
            onSelect={id => {
              onSelect(id);
              setOpen(false);
            }}
          />
        </div>
      ) : null}
    </div>
  );
}

/**
 * Shared legal reading template for `/legal/terms`, `/legal/privacy`, and `/legal/refunds`.
 * Note: `Draft for legal review` lives exclusively in the slim prototype top bar above the site
 * chrome, never inside the page body (`brief-lanes-round3.md`).
 */
export function LegalPage({
  initialDocId,
  mobileTocOpen = false,
  scrollToSectionId = null,
}: {
  initialDocId: LegalDocId;
  mobileTocOpen?: boolean;
  scrollToSectionId?: string | null;
}) {
  const [docId, setDocId] = useState<LegalDocId>(initialDocId);
  const doc = LEGAL_DOCUMENTS[docId];
  const [activeId, setActiveId] = useState<string>(scrollToSectionId ?? doc.toc[0]?.id ?? "");

  useLayoutEffect(() => {
    const scrollTarget = () => {
      if (!scrollToSectionId) {
        window.scrollTo({ top: 0, behavior: "instant" });
        return;
      }
      document
        .getElementById(scrollToSectionId)
        ?.scrollIntoView({ behavior: "instant", block: "start" });
    };
    scrollTarget();
    const frame = requestAnimationFrame(scrollTarget);
    const settle = window.setTimeout(scrollTarget, 50);
    void document.fonts?.ready.then(scrollTarget);
    return () => {
      cancelAnimationFrame(frame);
      window.clearTimeout(settle);
    };
  }, [scrollToSectionId, docId]);

  return (
    <div id="landing" className="rc-site-dark dark bg-[#0a0a0a] font-sans text-fg antialiased">
      {/* Prototype-only top bar: never part of the page body. */}
      <div
        role="note"
        className="fixed inset-x-0 top-0 z-[60] flex h-8 items-center justify-center border-b border-[color:var(--light-080)] bg-[#121212] px-4 font-mono text-[11.5px] text-muted-foreground"
      >
        Draft for legal review
      </div>

      <SiteHeader scrolled={Boolean(scrollToSectionId)} topOffsetClass="top-8" />

      <main className="docs-root pt-28 pb-24 md:pt-36">
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
                  {LEGAL_TABS.map(tabId => {
                    const item = LEGAL_DOCUMENTS[tabId];
                    const selected = tabId === docId;
                    return (
                      <button
                        key={tabId}
                        type="button"
                        aria-current={selected ? "page" : undefined}
                        onClick={() => {
                          setDocId(tabId);
                          setActiveId(item.toc[0]?.id ?? "");
                        }}
                        className={cn(
                          "cursor-pointer rounded-[8px] px-2.5 py-1 text-[13px] transition-colors",
                          selected
                            ? "bg-[color:var(--light-100)] text-fg"
                            : "text-muted-foreground hover:text-fg",
                          FOCUS
                        )}
                      >
                        {item.navLabel}
                      </button>
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

              <MobileTocDisclosure
                toc={doc.toc}
                defaultOpen={mobileTocOpen}
                activeId={activeId}
                onSelect={setActiveId}
              />
            </header>

            <div className="pt-9 [&_h2]:scroll-mt-32 [&_h3]:scroll-mt-32">
              <DocsProse text={doc.markdown} />
            </div>
          </article>

          <nav
            aria-label="On this page"
            className="docs-rail hidden max-h-[calc(100vh-152px)] overflow-y-auto pt-2 pb-10 xl:sticky xl:top-32 xl:block xl:self-start"
          >
            <p className="m-0 mb-4 text-[13px] text-subtle">On this page</p>
            <div className="relative">
              <span
                aria-hidden
                className="absolute inset-y-0 left-0 w-px bg-[color:var(--light-120)]"
              >
                <span aria-hidden className={cn("absolute inset-y-0 -left-px w-0.5", BEAM)} />
              </span>
              <LegalTocList toc={doc.toc} activeId={activeId} onSelect={setActiveId} />
            </div>
          </nav>
        </div>
      </main>

      <SiteFooter active={doc.navLabel} />
    </div>
  );
}
