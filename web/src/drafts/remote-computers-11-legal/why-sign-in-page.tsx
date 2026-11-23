import { cn, toast } from "@compozy/ui";
import { useRef } from "react";

import { DocsHeader } from "../docs/header";
import { DocMasthead } from "../docs/masthead";
import { DocsNavContext } from "../docs/nav-context";
import { DocsProse } from "../docs/prose";
import { DocsSidebar } from "../docs/sidebar";
import { BELOW_HEADER, LINE } from "../docs/text";
import { DocToc } from "../docs/toc";
import { SiteFooter } from "../landing/sections-close";
import { SIGN_IN_DOC_MARKDOWN, SIGN_IN_DOC_PAGE, SIGN_IN_DOC_TOC } from "./sign-in-doc-content";

/**
 * `/docs/account/sign-in` ("Why CompozyOS asks you to sign in") rendered inside the real CompozyOS
 * docs shell (`DocsHeader`, `DocsSidebar`, `DocMasthead`, `DocsProse`, `DocToc`, `SiteFooter`).
 */
export function WhySignInDocsPage() {
  const body = useRef<HTMLElement>(null);
  const handleNavigate = (slug: string) => {
    toast.message(slug ? `Navigating to /docs/${slug}` : "Navigating to /docs");
  };

  return (
    <div id="landing" className="rc-site-dark dark bg-[#0a0a0a] font-sans text-fg antialiased">
      <DocsNavContext value={handleNavigate}>
        <DocsHeader
          groupId="account"
          onMenu={() => toast.message("Opens mobile docs navigation")}
          onOpen={handleNavigate}
          onSearch={() => toast.message("Opens docs search (⌘K)")}
        />

        <main className="docs-root min-h-screen pt-[104px]">
          <div className="mx-auto grid max-w-[1600px] lg:grid-cols-[296px_minmax(0,1fr)]">
            <aside
              className={cn(
                "docs-rail sticky hidden h-[calc(100vh-104px)] overflow-y-auto border-r px-4 pt-8 pb-12 lg:block",
                BELOW_HEADER,
                LINE
              )}
            >
              <DocsSidebar page={SIGN_IN_DOC_PAGE} onOpen={handleNavigate} />
            </aside>

            <div className="grid gap-x-14 px-5 md:px-10 xl:grid-cols-[minmax(0,1fr)_224px] 2xl:gap-x-20">
              <article ref={body} className="min-w-0 max-w-[800px] pt-10 pb-24">
                <DocMasthead
                  facts="3 min read"
                  page={SIGN_IN_DOC_PAGE}
                  source={SIGN_IN_DOC_MARKDOWN}
                />
                <div className="pt-9">
                  <DocsProse text={SIGN_IN_DOC_MARKDOWN} />
                </div>
              </article>
              <DocToc body={body} toc={SIGN_IN_DOC_TOC} />
            </div>
          </div>
        </main>

        <SiteFooter active="Docs" />
      </DocsNavContext>
    </div>
  );
}
