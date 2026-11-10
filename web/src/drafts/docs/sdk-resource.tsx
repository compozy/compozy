/** One SDK resource: every method listed, then each with its call, its options and a working sample. */
import { Button, SearchInput, cn } from "@compozy/ui";
import { use, useState } from "react";

import {
  methodName,
  returnsOf,
  signatureOf,
  type SdkLang,
  type SdkMethod,
  type SdkResource,
  type SdkSamples,
} from "./sdk";
import { SdkLangContext } from "./sdk-context";
import { SdkFields, SdkSample } from "./sdk-parts";
import { BELOW_HEADER, LINE } from "./text";
import { METHOD_TONE } from "./api";

const PREVIEW = 10;
const TAG =
  "max-w-full rounded-[7px] border border-[color:var(--light-100)] px-2 py-0.5 font-mono text-[12px] break-all text-muted-foreground";

function sampleOf(samples: SdkSamples, method: SdkMethod, lang: SdkLang) {
  const sample = samples[method.id];
  return (lang === "go" ? sample?.go : sample?.typescript) ?? "";
}

function MethodIndex({ methods, lang }: { methods: readonly SdkMethod[]; lang: SdkLang }) {
  return (
    <ul className="m-0 mt-5 grid list-none gap-x-10 p-0 md:grid-cols-2 xl:grid-cols-3">
      {methods.map(method => (
        <li key={method.id} className={cn("border-b", LINE)}>
          <a
            href={`#${method.id}`}
            className="flex items-baseline gap-3 py-2.5 transition-colors hover:text-fg"
          >
            <code className="min-w-0 truncate font-mono text-[13.5px] text-fg">
              {methodName(method, lang)}
            </code>
            {method.kind === "json" ? null : (
              <span className="ml-auto shrink-0 font-mono text-[11.5px] text-brand-strong">
                {method.kind}
              </span>
            )}
          </a>
        </li>
      ))}
    </ul>
  );
}

function MethodEntry(props: {
  resource: SdkResource;
  method: SdkMethod;
  lang: SdkLang;
  sample: string;
}) {
  const { method, lang } = props;
  return (
    <section
      id={method.id}
      className={cn(
        "grid gap-x-14 gap-y-7 border-t py-14 lg:grid-cols-[minmax(0,1fr)_minmax(0,500px)]",
        LINE
      )}
    >
      <div className="min-w-0">
        <h2 className="docs-count m-0 font-mono text-[22px]/[1.3] font-normal break-words text-fg">
          {methodName(method, lang)}
        </h2>
        <p className="mt-2 mb-0 text-[16px]/[1.6] text-fg/80">{method.summary}</p>
        <p
          className={cn(
            "mt-6 mb-0 overflow-x-auto rounded-[12px] border bg-[#0d0d0d] px-4 py-3 font-mono text-[13.5px]/[1.6] whitespace-pre text-fg",
            LINE
          )}
        >
          {signatureOf(props.resource, method, lang)}
          <span className="text-subtle">{`  →  ${returnsOf(method, lang)}`}</span>
        </p>
        <SdkFields title="Options" fields={method.params} />
        {method.body ? (
          <p className="mt-6 mb-0 flex flex-wrap items-baseline gap-2.5 text-[14px] text-subtle">
            Request body
            <code className="font-mono text-[13.5px] text-fg">{method.body}</code>
          </p>
        ) : null}
        <p className="mt-7 mb-0 flex flex-wrap items-center gap-2">
          <span className={TAG}>
            <span className={METHOD_TONE[method.method] ?? "text-fg"}>{method.method}</span>{" "}
            {method.path}
          </span>
          <span className={TAG}>{method.app ? "app tokens allowed" : "app tokens denied"}</span>
        </p>
      </div>
      <div className={cn("min-w-0 lg:sticky lg:self-start lg:pt-16", BELOW_HEADER)}>
        {props.sample ? (
          <SdkSample
            code={props.sample}
            lang={lang}
            label={lang === "go" ? "main.go" : "example.ts"}
          />
        ) : null}
      </div>
    </section>
  );
}

export function SdkResourceBody({
  resource,
  samples,
}: {
  resource: SdkResource;
  samples: SdkSamples;
}) {
  const lang = use(SdkLangContext);
  const [query, setQuery] = useState("");
  const [all, setAll] = useState(false);
  const needle = query.trim().toLowerCase();
  const matched = needle
    ? resource.ops.filter(method => `${method.id} ${method.summary}`.toLowerCase().includes(needle))
    : resource.ops;
  const shown = all || needle ? matched : matched.slice(0, PREVIEW);
  return (
    <>
      <div className="mt-8 flex flex-wrap items-center justify-between gap-4">
        <SearchInput
          value={query}
          onChange={setQuery}
          placeholder={`Filter ${resource.ops.length} methods`}
          aria-label="Filter methods"
          containerClassName="h-10 w-full sm:w-[320px] [&_input]:text-[14px]"
        />
        <p className="m-0 font-mono text-[12.5px] text-subtle tabular-nums">
          {matched.length} of {resource.ops.length}
        </p>
      </div>
      <MethodIndex methods={matched} lang={lang} />
      <div className="mt-12">
        {shown.map(method => (
          <MethodEntry
            key={method.id}
            resource={resource}
            method={method}
            lang={lang}
            sample={sampleOf(samples, method, lang)}
          />
        ))}
      </div>
      {shown.length < matched.length ? (
        <Button variant="outline" size="sm" onClick={() => setAll(true)}>
          Show all {matched.length} methods
        </Button>
      ) : null}
    </>
  );
}
