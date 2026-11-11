/** The SDK reference: an overview, the client guides, and one page per resource. */
import { cn } from "@compozy/ui";
import { ArrowRight } from "@compozy/ui/icons";
import { use } from "react";

import { PageFrame, PageSkeleton } from "./article-page";
import { useSdk } from "./hooks/use-sdk";
import { DocMasthead, DocPager } from "./masthead";
import { slugify, type TocItem } from "./mdx";
import type { DocPage } from "./nav";
import { DocsNavContext } from "./nav-context";
import { DocsProse } from "./prose";
import type { SdkBundle, SdkData, SdkGuide, SdkLang, SdkResource } from "./sdk";
import { SdkLangContext } from "./sdk-context";
import { SdkFields, SdkLangBar, SdkSample, SdkSignature } from "./sdk-parts";
import { SdkResourceBody } from "./sdk-resource";
import { FOCUS, LINE } from "./text";

const HEADING =
  "mt-14 mb-4 font-display text-[27px]/[1.2] font-normal tracking-[-0.02em] text-balance text-fg";
const OVERVIEW = ["Install", "Create a session and prompt it", "Client", "Resources"];

function toc(titles: readonly string[]): TocItem[] {
  return titles.map(title => ({ id: slugify(title), text: title, depth: 2 }));
}

function GuideLinks({ guides }: { guides: readonly SdkGuide[] }) {
  const open = use(DocsNavContext);
  return (
    <ul className={cn("m-0 list-none divide-y divide-[color:var(--light-080)] border-y p-0", LINE)}>
      {guides.map(guide => (
        <li key={guide.slug}>
          <button
            type="button"
            onClick={() => open(guide.slug)}
            className={cn(
              "group grid w-full cursor-pointer gap-x-6 gap-y-1 bg-transparent py-4 text-left sm:grid-cols-[140px_minmax(0,1fr)_auto]",
              FOCUS
            )}
          >
            <span className="text-[16px] text-fg">{guide.title}</span>
            <span className="text-[14.5px]/[1.55] text-muted-foreground">{guide.description}</span>
            <ArrowRight
              className="size-4 self-center text-subtle transition-transform group-hover:translate-x-1 group-hover:text-fg"
              aria-hidden
            />
          </button>
        </li>
      ))}
    </ul>
  );
}

function ResourceGrid({ resources, lang }: { resources: readonly SdkResource[]; lang: SdkLang }) {
  const open = use(DocsNavContext);
  return (
    <ul className="m-0 grid list-none gap-3 p-0 sm:grid-cols-2 lg:grid-cols-3">
      {resources.map(resource => (
        <li key={resource.id}>
          <button
            type="button"
            onClick={() => open(resource.slug)}
            className={cn(
              "group flex w-full cursor-pointer flex-col gap-3 rounded-[14px] border border-[color:var(--light-090)] bg-[color:var(--light-025)] p-4 text-left transition-colors hover:border-[color:var(--light-200)] hover:bg-[color:var(--light-040)]",
              FOCUS
            )}
          >
            <span className="flex items-baseline justify-between gap-3 text-[15.5px] text-fg">
              {resource.title}
              <span className="font-mono text-[12px] text-subtle tabular-nums">
                {resource.ops.length}
              </span>
            </span>
            <code className="truncate font-mono text-[12.5px] text-muted-foreground">
              {lang === "go" ? "client.Client" : `compozy.${resource.id}`}
            </code>
          </button>
        </li>
      ))}
    </ul>
  );
}

function SdkOverview({ data }: { data: SdkData }) {
  const lang = use(SdkLangContext);
  return (
    <>
      <SdkLangBar />
      <h2 id={slugify(OVERVIEW[0] ?? "")} className={cn(HEADING, "mt-10")}>
        {OVERVIEW[0]}
      </h2>
      <SdkSample code={data.install[lang]} lang={lang} label="Terminal" />
      <h2 id={slugify(OVERVIEW[1] ?? "")} className={HEADING}>
        {OVERVIEW[1]}
      </h2>
      <SdkSample code={data.run[lang]} lang={lang} label={lang === "go" ? "main.go" : "run.mjs"} />
      <h2 id={slugify(OVERVIEW[2] ?? "")} className={HEADING}>
        {OVERVIEW[2]}
      </h2>
      <GuideLinks guides={data.guides} />
      <h2 id={slugify(OVERVIEW[3] ?? "")} className={HEADING}>
        {OVERVIEW[3]}
        <span className="docs-count ml-3 font-mono text-[13px] text-subtle tabular-nums">
          {data.stats.resources}
        </span>
      </h2>
      <ResourceGrid resources={data.resources} lang={lang} />
    </>
  );
}

function SdkGuideBody({ guide }: { guide: SdkGuide }) {
  const lang = use(SdkLangContext);
  return (
    <>
      <SdkLangBar />
      {guide.sections.map(section => {
        const variant = section[lang];
        return (
          <section key={section.title}>
            <h2 id={slugify(section.title)} className={HEADING}>
              {section.title}
            </h2>
            <DocsProse text={section.text} className="mb-6" />
            {variant ? (
              <>
                <SdkSignature code={variant.signature} lang={lang} />
                <SdkFields title={lang === "go" ? "Fields" : "Options"} fields={variant.fields} />
                {variant.example ? (
                  <div className="mt-7">
                    <SdkSample code={variant.example} lang={lang} label="Example" />
                  </div>
                ) : null}
              </>
            ) : (
              <p className="m-0 rounded-[12px] border border-[color:var(--light-090)] px-4 py-3 text-[14.5px] text-muted-foreground">
                TypeScript only. The Go client has no equivalent.
              </p>
            )}
          </section>
        );
      })}
    </>
  );
}

function SdkResourcePage({
  page,
  resource,
  bundle,
}: {
  page: DocPage;
  resource: SdkResource;
  bundle: SdkBundle;
}) {
  const allowed = resource.ops.filter(method => method.app).length;
  return (
    <div className="min-w-0 px-5 pt-10 pb-24 md:px-10">
      <div className="max-w-[860px]">
        <DocMasthead
          page={page}
          source=""
          facts={`${resource.ops.length} methods · ${allowed} available to app tokens`}
        />
      </div>
      <SdkLangBar />
      <SdkResourceBody resource={resource} samples={bundle.samples} />
      <div className="max-w-[860px]">
        <DocPager slug={page.slug} />
      </div>
    </div>
  );
}

export function SdkPage({ page }: { page: DocPage }) {
  const bundle = useSdk();
  if (!bundle) return <PageSkeleton />;
  const { data } = bundle;
  const resource = data.resources.find(candidate => candidate.slug === page.slug);
  if (resource) return <SdkResourcePage page={page} resource={resource} bundle={bundle} />;
  const guide = data.guides.find(candidate => candidate.slug === page.slug);
  if (guide) {
    return (
      <PageFrame page={page} source="" toc={toc(guide.sections.map(section => section.title))}>
        <SdkGuideBody guide={guide} />
      </PageFrame>
    );
  }
  return (
    <PageFrame
      page={page}
      source=""
      facts={`${data.stats.operations} operations · ${data.stats.resources} resources · ${data.stats.app} available to app tokens`}
      toc={toc(OVERVIEW)}
    >
      <SdkOverview data={data} />
    </PageFrame>
  );
}
