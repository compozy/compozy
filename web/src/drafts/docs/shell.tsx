/** The docs app shell: chrome, sidebar, the open page, and the two overlays. */
import { cn } from "@compozy/ui";
import { XIcon } from "@compozy/ui/icons";
import { useState } from "react";

import { ApiPage } from "./api-page";
import { ArticlePage, PageSkeleton } from "./article-page";
import { isCliPage } from "./cli";
import { CliPage } from "./cli-page";
import { DocsHeader } from "./header";
import { DocsHome } from "./home-page";
import { useDoc } from "./hooks/use-doc";
import { useDocsKeys } from "./hooks/use-docs-keys";
import { findPage, type DocPage } from "./nav";
import { DocsNavContext } from "./nav-context";
import { isSdkSlug } from "./sdk";
import { SdkLangContext, SdkSetLangContext } from "./sdk-context";
import { SdkPage } from "./sdk-page";
import { DocsSearch } from "./search";
import { DocsSidebar } from "./sidebar";
import { BELOW_HEADER, FOCUS, LINE } from "./text";
import { withSearch, withoutOverlays, type DocsView } from "./view";

function DocView({ page, onSearch }: { page: DocPage; onSearch: () => void }) {
  if (isSdkSlug(page.slug)) return <SdkPage page={page} />;
  return <MdxView page={page} onSearch={onSearch} />;
}

function MdxView({ page, onSearch }: { page: DocPage; onSearch: () => void }) {
  const loaded = useDoc(page.slug);
  if (page.slug === "")
    return <DocsHome page={page} doc={loaded?.doc ?? null} onSearch={onSearch} />;
  if (!loaded) return <PageSkeleton />;
  if (!loaded.doc) return <PageSkeleton />;
  if (page.slug === "api/sessions") return <ApiPage page={page} doc={loaded.doc} />;
  if (isCliPage(page.slug, loaded.doc.source)) return <CliPage page={page} doc={loaded.doc} />;
  return <ArticlePage page={page} doc={loaded.doc} />;
}

function MobileDrawer(props: {
  page: DocPage;
  onOpen: (slug: string) => void;
  onClose: () => void;
}) {
  return (
    <div className="fixed inset-0 z-50 lg:hidden">
      <button
        type="button"
        aria-label="Close navigation"
        onClick={props.onClose}
        className="absolute inset-0 cursor-default bg-[color:var(--dark-700)] backdrop-blur-sm"
      />
      <div
        className={cn(
          "docs-rail absolute inset-y-0 left-0 w-[min(340px,88vw)] overflow-y-auto border-r bg-[#0c0c0c] px-4 pt-4 pb-10",
          LINE
        )}
      >
        <button
          type="button"
          aria-label="Close navigation"
          onClick={props.onClose}
          className={cn(
            "mb-4 ml-auto flex size-9 cursor-pointer items-center justify-center rounded-[10px] border border-[color:var(--light-120)] bg-transparent text-fg/80",
            FOCUS
          )}
        >
          <XIcon className="size-4" aria-hidden />
        </button>
        <DocsSidebar page={props.page} onOpen={props.onOpen} />
      </div>
    </div>
  );
}

export function DocsShell({ initialView }: { initialView: DocsView }) {
  const [view, setView] = useState(initialView);
  useDocsKeys(setView);
  const page = findPage(view.slug);
  const open = (slug: string) => {
    setView(current => ({ ...current, slug, query: null, menu: false }));
    window.scrollTo({ top: 0 });
  };
  const search = () => setView(withSearch);
  return (
    <DocsNavContext value={open}>
      <SdkLangContext value={view.lang}>
        <SdkSetLangContext value={lang => setView(current => ({ ...current, lang }))}>
          <DocsHeader
            groupId={page.group?.id ?? null}
            onOpen={open}
            onSearch={search}
            onMenu={() => setView(current => ({ ...current, menu: true, query: null }))}
          />
          <div className="docs-root min-h-screen pt-[104px]">
            {page.slug === "" ? (
              <DocView key="home" page={page} onSearch={search} />
            ) : (
              <div className="mx-auto grid max-w-[1600px] lg:grid-cols-[296px_minmax(0,1fr)]">
                <aside
                  className={cn(
                    "docs-rail sticky hidden h-[calc(100vh-104px)] overflow-y-auto border-r px-4 pt-8 pb-12 lg:block",
                    BELOW_HEADER,
                    LINE
                  )}
                >
                  <DocsSidebar page={page} onOpen={open} />
                </aside>
                <DocView key={page.slug} page={page} onSearch={search} />
              </div>
            )}
          </div>
          {view.menu ? (
            <MobileDrawer page={page} onOpen={open} onClose={() => setView(withoutOverlays)} />
          ) : null}
          {view.query === null ? null : (
            <DocsSearch
              query={view.query}
              onQuery={query => setView(current => ({ ...current, query }))}
              onClose={() => setView(withoutOverlays)}
              onOpen={open}
            />
          )}
        </SdkSetLangContext>
      </SdkLangContext>
    </DocsNavContext>
  );
}
