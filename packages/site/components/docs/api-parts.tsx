"use client";

/**
 * The parts of the API resource template (`web/src/drafts/docs/api-page.tsx`) that
 * `fumadocs-openapi` has no slot markup for. Each reads the page's own OpenAPI document through the
 * library's hooks, so nothing here keeps a second copy of the API.
 */
import { Button, Disclosure, Tabs, TabsContent, TabsList, TabsTrigger, cn } from "@compozy/ui";
import { useComponents, useOpenAPI, type OperationItem } from "fumadocs-openapi";
import { useCodeUsage, useOperation, type ResponseTab } from "fumadocs-openapi/operation";
import { useState, type ComponentProps, type ReactNode } from "react";

import { BELOW_HEADER, FOCUS, LINE } from "@/components/docs/docs-classes";
import { CodeFrame } from "@/components/docs/docs-prose";
import { useOpenAtFragment } from "@/components/docs/hooks/use-open-at-fragment";
import { operationAnchor } from "@/lib/api-reference";

const PREVIEW = 12;
const TAB = "h-8 rounded-[9px] px-3 text-[13px]";
const LABEL = "m-0 mb-1 flex items-baseline gap-3 text-[15px] font-normal text-fg";

const METHOD_TONE: Record<string, string> = {
  GET: "text-brand-strong",
  POST: "text-success",
  PUT: "text-warning",
  PATCH: "text-warning",
  DELETE: "text-destructive",
};

/** The library's section titles, in the draft's sentence case. */
const SECTION_TITLES: Record<string, string> = {
  "Path Parameters": "Path parameters",
  "Query Parameters": "Query parameters",
  "Header Parameters": "Header parameters",
  "Cookie Parameters": "Cookie parameters",
  "Request Body": "Request body",
  "Response Body": "Response body",
};

export function ApiMethod({ method }: { method: string }) {
  const upper = method.toUpperCase();
  return (
    <span className={cn("font-mono text-[12.5px] font-medium", METHOD_TONE[upper] ?? "text-fg")}>
      {upper}
    </span>
  );
}

/** Headings of the library's sections: the endpoint summary as h2, its parts as quiet labels. */
export function ApiHeading({
  id,
  depth,
  children,
  className: _className,
  ...props
}: ComponentProps<"h1"> & { id: string; depth: number }) {
  if (depth <= 2) {
    return (
      <h2
        {...props}
        id={id}
        className="m-0 font-display text-[27px]/[1.2] font-normal tracking-[-0.02em] text-balance text-fg"
      >
        {children}
      </h2>
    );
  }
  const title = typeof children === "string" ? (SECTION_TITLES[children] ?? children) : children;
  return (
    <h3 {...props} id={id} className={LABEL}>
      {title}
    </h3>
  );
}

interface IndexEntry {
  key: string;
  method: string;
  path: string;
  summary: string;
}

/** Every endpoint of the resource, linking to its section; the first twelve until asked. */
export function EndpointIndex({ items }: { items: readonly OperationItem[] }) {
  const { doc } = useOpenAPI();
  const [all, setAll] = useState(false);
  const entries: IndexEntry[] = items.map(item => {
    const operation = doc.bundled.paths?.[item.path]?.[item.method];
    return {
      key: `${item.method}:${item.path}`,
      method: item.method,
      path: item.path,
      summary: operation?.summary ?? operation?.operationId ?? item.path,
    };
  });
  const shown = all ? entries : entries.slice(0, PREVIEW);
  return (
    <section aria-label="Endpoints" className="mt-10 max-w-[860px]">
      <p className="m-0 mb-3 flex items-baseline gap-3 text-[15px] text-fg">
        Endpoints
        <span className="font-mono text-[13px] text-subtle tabular-nums">{entries.length}</span>
      </p>
      <ul className={cn("m-0 list-none divide-y divide-site-light-080 border-y p-0", LINE)}>
        {shown.map(entry => (
          <li key={entry.key}>
            <a
              href={`#${operationAnchor(entry.method, entry.path)}`}
              className={cn(
                "grid items-baseline gap-x-5 gap-y-1 py-3 transition-colors hover:bg-site-light-025 md:grid-cols-[56px_minmax(0,0.9fr)_minmax(0,1fr)]",
                FOCUS
              )}
            >
              <ApiMethod method={entry.method} />
              <code className="min-w-0 font-mono text-[13.5px] break-words text-fg">
                {entry.path}
              </code>
              <span className="text-[14px]/[1.5] text-muted-foreground">{entry.summary}</span>
            </a>
          </li>
        ))}
      </ul>
      {all || entries.length <= PREVIEW ? null : (
        <Button variant="outline" size="sm" className="mt-4" onClick={() => setAll(true)}>
          Show all {entries.length} endpoints
        </Button>
      )}
    </section>
  );
}

/** The method and path chip that opens each endpoint. */
export function EndpointChip({ method, path }: { method: string; path: string }) {
  return (
    <p className="m-0 flex flex-wrap items-center gap-3 rounded-[12px] border border-site-light-100 bg-site-light-025 px-4 py-2.5">
      <ApiMethod method={method} />
      <code className="min-w-0 font-mono text-[14px] break-all text-fg">{path}</code>
    </p>
  );
}

export function Transports({ transports }: { transports: unknown }) {
  if (!Array.isArray(transports) || transports.length === 0) return null;
  return (
    <p className="m-0 mt-3 flex flex-wrap gap-2 text-[13px] text-subtle">
      {transports.map(transport => (
        <span
          key={String(transport)}
          className="rounded-[7px] border border-site-light-100 px-2 py-0.5 font-mono"
        >
          {String(transport)}
        </span>
      ))}
    </p>
  );
}

/**
 * Status codes and what they mean, as the draft lists them. The library's per-status response
 * schema stays one click away below the list, so no field of the current page is lost.
 */
export function ApiResponses({ schema }: { schema: ReactNode }) {
  const { responses } = useOperation();
  const [schemaOpen, setSchemaOpen] = useState(false);
  const frame = useOpenAtFragment<HTMLDivElement>(schemaOpen, setSchemaOpen);
  if (responses.length === 0) return null;
  return (
    <div ref={frame} className="mt-9">
      <p className={LABEL}>Responses</p>
      <dl className={cn("m-0 divide-y divide-site-light-080 border-y", LINE)}>
        {responses.map(({ status, response }) => (
          <div key={status} className="grid grid-cols-[52px_minmax(0,1fr)] gap-4 py-3">
            <dt
              className={cn(
                "font-mono text-[13.5px]",
                status.startsWith("2") ? "text-success" : "text-subtle"
              )}
            >
              {status}
            </dt>
            <dd className="m-0 text-[14.5px]/[1.55] text-fg/75">{response.description}</dd>
          </div>
        ))}
      </dl>
      {schema ? (
        <Disclosure
          label="Response schema"
          size="md"
          // Findable while closed, as the `<details>` it replaces was: find-in-page and a link to
          // an id inside the schema open it.
          hiddenUntilFound
          open={schemaOpen}
          onOpenChange={setSchemaOpen}
          className="mt-3"
          triggerProps={{ className: "h-8 py-0 text-[13.5px] font-normal" }}
          contentProps={{ className: "api-response-schema pt-3" }}
        >
          {schema}
        </Disclosure>
      ) : null}
    </div>
  );
}

/** Shiki through the page's own highlighter, inside the docs code frame. */
function HighlightedCode({ code, lang }: { code: string; lang: string }) {
  const { CodeBlock } = useComponents();
  return (
    <CodeBlock
      lang={lang}
      code={code}
      codeblock={{
        allowCopy: false,
        keepBackground: false,
        className: "api-code m-0 rounded-none border-0 bg-transparent shadow-none",
      }}
    />
  );
}

function SampleCode({ id, lang }: { id: string; lang: string }) {
  const code = useCodeUsage(id) ?? "";
  return (
    <CodeFrame code={code} label="Request" shell={lang === "bash"}>
      <HighlightedCode code={code} lang={lang} />
    </CodeFrame>
  );
}

export interface SampleTab {
  id: string;
  label: string;
  lang: string;
}

/** Request samples as tabs: cURL and the generated SDK calls, in the registry's order. */
export function SampleTabs({ samples }: { samples: readonly SampleTab[] }) {
  const first = samples[0];
  if (!first) return null;
  return (
    // Explicit `flex-col` and `h-auto`: the site does not ship the custom variants the Tabs use.
    <Tabs defaultValue={first.id} className="flex-col gap-0">
      <TabsList className="h-auto flex-wrap gap-1 border-b-0">
        {samples.map(sample => (
          <TabsTrigger key={sample.id} value={sample.id} className={TAB}>
            {sample.label}
          </TabsTrigger>
        ))}
      </TabsList>
      {samples.map(sample => (
        <TabsContent key={sample.id} value={sample.id} className="[&>div]:mt-3 [&>div]:mb-0">
          <SampleCode id={sample.id} lang={sample.lang} />
        </TabsContent>
      ))}
    </Tabs>
  );
}

/** The response example, opening on the first success; other statuses with examples switch it. */
export function ResponseExample({ tabs }: { tabs: readonly ResponseTab[] }) {
  const withExamples = tabs.filter(tab => tab.examples?.length);
  const fallback = withExamples.find(tab => tab.code.startsWith("2")) ?? withExamples[0];
  const [selected, setSelected] = useState(fallback?.code);
  const tab = withExamples.find(item => item.code === selected) ?? fallback;
  if (!tab) return null;
  const code = JSON.stringify(tab.examples?.[0]?.sample, null, 2) ?? "";
  return (
    <div className="[&>div]:mt-4 [&>div]:mb-0">
      <CodeFrame code={code} label={`${tab.code} ${tab.response.description ?? ""}`} shell={false}>
        {withExamples.length > 1 ? (
          <div
            role="group"
            aria-label="Response status"
            className={cn("flex flex-wrap gap-1 border-b px-3 py-2", LINE)}
          >
            {withExamples.map(item => (
              <button
                key={item.code}
                type="button"
                aria-pressed={item.code === tab.code}
                onClick={() => setSelected(item.code)}
                className={cn(
                  "h-6 rounded-[7px] px-2 font-mono text-[12px] transition-colors",
                  item.code === tab.code
                    ? "bg-site-light-080 text-fg"
                    : "text-subtle hover:text-fg",
                  FOCUS
                )}
              >
                {item.code}
              </button>
            ))}
          </div>
        ) : null}
        <HighlightedCode code={code} lang="json" />
      </CodeFrame>
    </div>
  );
}

/** The right column: samples, then the response, held under the header while the endpoint scrolls. */
export function ApiExampleColumn({ children }: { children: ReactNode }) {
  return (
    <div className={cn("min-w-0 xl:sticky xl:self-start xl:pt-6", BELOW_HEADER)}>{children}</div>
  );
}
