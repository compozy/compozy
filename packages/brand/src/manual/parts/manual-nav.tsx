// Sticky header: lockup, chapter nav, and the in-page section index with scroll-spy.
import { Link } from "@tanstack/react-router";
import { useEffect, useRef } from "react";

import { EDITION, hrefOf, pages, VERSION, type Chapter } from "../chapters";

function centerChild(list: HTMLElement | null, child: Element | null) {
  if (!list || !(child instanceof HTMLElement)) return;
  list.scrollLeft = child.offsetLeft - list.clientWidth / 2 + child.clientWidth / 2;
}

export function ManualNav({
  page,
  activeSection,
}: {
  page: Chapter;
  activeSection: string | null;
}) {
  const chaptersRef = useRef<HTMLOListElement>(null);
  const tocRef = useRef<HTMLOListElement>(null);
  useEffect(() => {
    centerChild(
      chaptersRef.current,
      chaptersRef.current?.querySelector("[aria-current=page]") ?? null
    );
  }, [page]);
  useEffect(() => {
    if (!activeSection) return;
    centerChild(
      tocRef.current,
      tocRef.current?.querySelector(`a[href="#${activeSection}"]`) ?? null
    );
  }, [activeSection]);

  return (
    <nav
      aria-label="Brand manual"
      className="sticky top-0 z-30 border-b border-line bg-[color-mix(in_oklab,var(--color-rail)_86%,transparent)] backdrop-blur-xl"
    >
      <div className="wrap flex min-h-14 items-center gap-6">
        <a
          href="/"
          aria-label="Manual home"
          className="flex flex-none items-center gap-3 no-underline"
        >
          <img
            src="/brand/svg/compozy-lockup-dark.svg"
            alt="Compozy"
            className="h-[22px] w-auto"
            width="140"
            height="22"
          />
          <span className="mono-label text-[11px] tracking-[0.12em] text-muted-foreground">
            brand
          </span>
          <span className="hidden font-mono text-[11px] text-muted-foreground xl:inline">
            {VERSION} · {EDITION}
          </span>
        </a>
        <ol
          ref={chaptersRef}
          className="-mx-2 flex min-w-0 list-none gap-0.5 overflow-x-auto p-0 [scrollbar-width:none] lg:mx-0 lg:ml-auto [&::-webkit-scrollbar]:hidden"
        >
          {pages.map(p => (
            <li key={p.n}>
              <Link
                to={hrefOf(p)}
                aria-current={p === page ? "page" : undefined}
                className="block rounded-lg px-2.5 py-1.5 text-[13.5px] font-medium whitespace-nowrap text-muted-foreground no-underline hover:text-fg aria-[current=page]:bg-elevated aria-[current=page]:text-(--color-brand-cyan)"
              >
                {p.title}
              </Link>
            </li>
          ))}
        </ol>
      </div>
      <div className="border-t border-line">
        <ol
          ref={tocRef}
          aria-label="Sections on this page"
          className="wrap flex min-w-0 list-none gap-0.5 overflow-x-auto py-1.5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        >
          {page.sections.map(([id, name], k) => (
            <li key={id}>
              <a
                href={`#${id}`}
                aria-current={activeSection === id ? "true" : undefined}
                className="flex items-center gap-1.5 rounded-lg px-2.5 py-1 text-[12.5px] font-medium whitespace-nowrap text-muted-foreground no-underline hover:text-fg aria-[current=true]:text-(--color-brand-cyan)"
              >
                <span className="font-mono text-[10.5px] opacity-60">
                  {String(k + 1).padStart(2, "0")}
                </span>
                {name}
              </a>
            </li>
          ))}
        </ol>
      </div>
    </nav>
  );
}
