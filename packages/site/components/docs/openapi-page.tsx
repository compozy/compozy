"use client";

import { cn } from "@compozy/ui";
import { COMPOZY_CODE_THEMES } from "@compozy/ui/lib/code-theme";
import { defaultShikiFactory } from "fumadocs-core/highlight/shiki/full";
import type { OperationObject } from "fumadocs-openapi";
import { createCodeUsageGeneratorRegistry } from "fumadocs-openapi/requests/generators";
import { curl } from "fumadocs-openapi/requests/generators/curl";
import { python } from "fumadocs-openapi/requests/generators/python";
import { createOpenAPIPage } from "fumadocs-openapi/ui";

import {
  ApiExampleColumn,
  ApiHeading,
  ApiResponses,
  EndpointChip,
  EndpointIndex,
  ResponseExample,
  SampleTabs,
  Transports,
} from "@/components/docs/api-parts";
import { LINE } from "@/components/docs/docs-classes";
import { operationAnchor } from "@/lib/api-reference";

export type { OpenAPIPageProps_Preloaded } from "fumadocs-openapi/ui";

/** The document's own extension: which daemon transports serve the operation. */
type CompozyOperation = OperationObject & { "x-compozy-transports"?: string[] };

const codeUsages = createCodeUsageGeneratorRegistry();
codeUsages.add("curl", curl);
// Per-operation x-codeSamples from the server replace or remove these reserved positions.
codeUsages.add("sdk-typescript", { lang: "ts", label: "TypeScript SDK", generate: () => "" });
codeUsages.add("sdk-go", { lang: "go", label: "Go SDK", generate: () => "" });
codeUsages.add("python", python);

/**
 * The API resource template of the docs draft, laid over `fumadocs-openapi`: the library still
 * resolves the document, parameters, schemas and samples; these slots only arrange and restyle them
 * (`app/styles/docs-api.css` owns the rest).
 */
export const OpenAPIPage = createOpenAPIPage({
  shiki: defaultShikiFactory,
  shikiOptions: {
    themes: { light: COMPOZY_CODE_THEMES.light, dark: COMPOZY_CODE_THEMES.dark },
  },
  playground: { enabled: false },
  codeUsages,
  components: { Heading: ApiHeading },
  content: {
    renderPageLayout: slots => (
      <div className="site-api-page">
        <EndpointIndex items={(slots.operations ?? []).map(op => op.item)} />
        <div className="mt-14">
          {slots.operations?.map(op => (
            <div key={`${op.item.path}:${op.item.method}`}>{op.children}</div>
          ))}
          {slots.webhooks?.map(hook => (
            <div key={`${hook.item.name}:${hook.item.method}`}>{hook.children}</div>
          ))}
        </div>
      </div>
    ),
    renderOperationLayout: (slots, { path, method, operation }) => (
      <section
        id={operationAnchor(method, path)}
        className={cn(
          "site-api-endpoint grid gap-x-14 gap-y-8 border-t py-14 lg:grid-cols-[minmax(0,1fr)_minmax(0,500px)]",
          LINE
        )}
      >
        <div className="min-w-0">
          <EndpointChip method={method} path={path} />
          <div className="mt-6">{slots.header}</div>
          <Transports transports={(operation as CompozyOperation)["x-compozy-transports"]} />
          {slots.description ? (
            <div className="mt-4 text-[15px]/[1.6] text-fg/75">{slots.description}</div>
          ) : null}
          {slots.authSchemes}
          {slots.parameters}
          {slots.body}
          <ApiResponses schema={slots.responses} />
          {slots.callbacks}
        </div>
        {slots.apiExample}
      </section>
    ),
    renderAPIExampleLayout: slots => (
      <ApiExampleColumn>
        {slots.selector}
        {slots.usageTabs}
        {slots.responseTabs}
      </ApiExampleColumn>
    ),
    renderAPIExampleUsageTabs: generators => (
      <SampleTabs
        samples={Array.from(generators.map(), ([id, generator]) => ({
          id,
          label: generator.label ?? generator.lang,
          lang: generator.lang,
        }))}
      />
    ),
    renderResponseTabs: ({ tabs }) => <ResponseExample tabs={tabs} />,
  },
});
