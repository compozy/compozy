/** An API resource: every endpoint listed, then each one explained beside its request and response. */
import { Button, Tabs, TabsContent, TabsList, TabsTrigger, cn } from "@compozy/ui";
import { useState } from "react";

import { DETAILED, METHOD_TONE, OPERATIONS, type ApiField, type ApiOperation } from "./api";
import { DocMasthead, DocPager } from "./masthead";
import type { Doc } from "./mdx";
import type { DocPage } from "./nav";
import { DocsCode } from "./prose";
import { BELOW_HEADER, LINE } from "./text";

const PREVIEW = 12;
const TAB = "h-8 rounded-[9px] px-3 text-[13px]";

function Method({ method }: { method: string }) {
  return (
    <span className={cn("font-mono text-[12.5px] font-medium", METHOD_TONE[method] ?? "text-fg")}>
      {method}
    </span>
  );
}

function EndpointIndex() {
  const [all, setAll] = useState(false);
  const shown = all ? OPERATIONS : OPERATIONS.slice(0, PREVIEW);
  return (
    <section aria-label="Endpoints" className="mt-10">
      <p className="m-0 mb-3 flex items-baseline gap-3 text-[15px] text-fg">
        Endpoints
        <span className="font-mono text-[13px] text-subtle tabular-nums">{OPERATIONS.length}</span>
      </p>
      <ul
        className={cn("m-0 list-none divide-y divide-[color:var(--light-080)] border-y p-0", LINE)}
      >
        {shown.map(operation => (
          <li key={operation.id}>
            <a
              href={`#${operation.id}`}
              className="grid items-baseline gap-x-5 gap-y-1 py-3 transition-colors hover:bg-[color:var(--light-025)] md:grid-cols-[56px_minmax(0,0.9fr)_minmax(0,1fr)]"
            >
              <Method method={operation.method} />
              <code className="min-w-0 font-mono text-[13.5px] break-words text-fg">
                {operation.path}
              </code>
              <span className="text-[14px]/[1.5] text-muted-foreground">{operation.summary}</span>
            </a>
          </li>
        ))}
      </ul>
      {all ? null : (
        <Button variant="outline" size="sm" className="mt-4" onClick={() => setAll(true)}>
          Show all {OPERATIONS.length} endpoints
        </Button>
      )}
    </section>
  );
}

function Fields({
  title,
  type,
  fields,
}: {
  title: string;
  type?: string;
  fields: readonly ApiField[];
}) {
  if (fields.length === 0) return null;
  return (
    <div className="mt-9">
      <p className="m-0 mb-1 flex items-baseline gap-3 text-[15px] text-fg">
        {title}
        {type ? <span className="font-mono text-[12.5px] text-subtle">{type}</span> : null}
      </p>
      <dl className={cn("m-0 divide-y divide-[color:var(--light-080)] border-y", LINE)}>
        {fields.map(field => (
          <div key={field.name} className="py-3.5">
            <dt className="flex flex-wrap items-center gap-2.5 font-mono text-[13.5px]">
              <span className="text-fg">{field.name}</span>
              <span className="text-subtle">{field.type}</span>
              {field.required ? <span className="text-[12px] text-warning">required</span> : null}
            </dt>
            {field.description ? (
              <dd className="m-0 mt-1.5 text-[14.5px]/[1.55] text-fg/75">{field.description}</dd>
            ) : null}
          </div>
        ))}
      </dl>
    </div>
  );
}

function Responses({ operation }: { operation: ApiOperation }) {
  return (
    <div className="mt-9">
      <p className="m-0 mb-1 text-[15px] text-fg">Responses</p>
      <dl className={cn("m-0 divide-y divide-[color:var(--light-080)] border-y", LINE)}>
        {(operation.responses ?? []).map(response => (
          <div key={response.code} className="grid grid-cols-[52px_minmax(0,1fr)] gap-4 py-3">
            <dt
              className={cn(
                "font-mono text-[13.5px]",
                response.code.startsWith("2") ? "text-success" : "text-subtle"
              )}
            >
              {response.code}
            </dt>
            <dd className="m-0 text-[14.5px]/[1.55] text-fg/75">{response.description}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function Samples({ operation }: { operation: ApiOperation }) {
  const samples = [
    { id: "curl", label: "cURL", language: "bash", code: operation.samples?.curl ?? "" },
    {
      id: "ts",
      label: "TypeScript SDK",
      language: "ts",
      code: operation.samples?.typescript ?? "",
    },
    { id: "go", label: "Go SDK", language: "go", code: operation.samples?.go ?? "" },
  ].filter(sample => sample.code);
  const ok = operation.responses?.find(response => response.code.startsWith("2"));
  return (
    <div className={cn("min-w-0 lg:sticky lg:self-start", BELOW_HEADER, "lg:pt-6")}>
      <Tabs defaultValue="curl">
        <TabsList className="h-auto gap-1 border-b-0">
          {samples.map(sample => (
            <TabsTrigger key={sample.id} value={sample.id} className={TAB}>
              {sample.label}
            </TabsTrigger>
          ))}
        </TabsList>
        {samples.map(sample => (
          <TabsContent key={sample.id} value={sample.id} className="[&>div]:mt-3 [&>div]:mb-0">
            <DocsCode code={sample.code} language={sample.language} label="Request" />
          </TabsContent>
        ))}
      </Tabs>
      {operation.response ? (
        <div className="[&>div]:mt-4 [&>div]:mb-0">
          <DocsCode
            code={operation.response}
            language="json"
            label={`${ok?.code ?? "200"} ${ok?.description ?? "Response"}`}
          />
        </div>
      ) : null}
    </div>
  );
}

function Endpoint({ operation }: { operation: ApiOperation }) {
  const params = operation.params ?? [];
  return (
    <section
      id={operation.id}
      className={cn(
        "grid gap-x-14 gap-y-8 border-t py-14 lg:grid-cols-[minmax(0,1fr)_minmax(0,500px)]",
        LINE
      )}
    >
      <div className="min-w-0">
        <p className="m-0 flex flex-wrap items-center gap-3 rounded-[12px] border border-[color:var(--light-100)] bg-[color:var(--light-025)] px-4 py-2.5">
          <Method method={operation.method} />
          <code className="min-w-0 font-mono text-[14px] break-words text-fg">
            {operation.path}
          </code>
        </p>
        <h2 className="mt-6 font-display text-[27px]/[1.2] font-normal tracking-[-0.02em] text-balance text-fg">
          {operation.summary}
        </h2>
        <p className="m-0 mt-3 flex flex-wrap gap-2 text-[13px] text-subtle">
          {(operation.transports ?? []).map(transport => (
            <span
              key={transport}
              className="rounded-[7px] border border-[color:var(--light-100)] px-2 py-0.5 font-mono"
            >
              {transport}
            </span>
          ))}
        </p>
        <Fields title="Path parameters" fields={params.filter(field => field.in === "path")} />
        <Fields title="Query parameters" fields={params.filter(field => field.in === "query")} />
        <Fields title="Request body" type={operation.bodyType} fields={operation.body ?? []} />
        <Responses operation={operation} />
      </div>
      <Samples operation={operation} />
    </section>
  );
}

export function ApiPage({ page, doc }: { page: DocPage; doc: Doc }) {
  return (
    <div className="min-w-0 px-5 pt-10 pb-24 md:px-10">
      <div className="max-w-[860px]">
        <DocMasthead page={page} source={doc.source} facts={`${OPERATIONS.length} endpoints`} />
        <EndpointIndex />
      </div>
      <div className="mt-14">
        {DETAILED.map(operation => (
          <Endpoint key={operation.id} operation={operation} />
        ))}
      </div>
      <div className="max-w-[860px]">
        <DocPager slug={page.slug} />
      </div>
    </div>
  );
}
