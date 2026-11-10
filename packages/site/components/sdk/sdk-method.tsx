"use client";

/** One method of a resource page: its call, its options and route, and the sample beside them. */
import { CodeBlock, cn } from "@compozy/ui";

import { BELOW_HEADER, LINE } from "@/components/docs/docs-classes";
import { CodeFrame } from "@/components/docs/docs-prose";
import { SdkFieldRow, SdkFields } from "@/components/sdk/sdk-symbol";
import {
  bodyOf,
  methodName,
  returnsOf,
  signatureOf,
  type SdkLang,
  type SdkMethod,
  type SdkResource,
} from "@/lib/sdk-reference-model";

const METHOD_TONE: Record<string, string> = {
  GET: "text-brand-strong",
  POST: "text-success",
  PUT: "text-warning",
  PATCH: "text-warning",
  DELETE: "text-destructive",
};

const TAG =
  "max-w-full rounded-[7px] border border-site-light-100 px-2 py-0.5 font-mono text-[12px] break-all text-muted-foreground";

/** A code sample in the docs frame, highlighted in the browser only when it is shown. */
export function SdkSample({ code, lang, label }: { code: string; lang: SdkLang; label: string }) {
  return (
    <CodeFrame code={code} label={label} shell={false}>
      <CodeBlock
        code={code}
        language={lang === "go" ? "go" : "ts"}
        caption=""
        copyable={false}
        className="rounded-none border-0 bg-transparent text-[13.5px]"
      />
    </CodeFrame>
  );
}

function CallShape({
  resource,
  method,
  lang,
}: {
  resource: SdkResource;
  method: SdkMethod;
  lang: SdkLang;
}) {
  const signature = signatureOf(resource, method, lang);
  const returns = returnsOf(method, lang);
  return (
    <p
      className={cn(
        "mt-6 mb-0 overflow-x-auto rounded-[12px] border bg-[#0d0d0d] px-4 py-3 font-mono text-[13.5px]/[1.6] whitespace-pre text-fg",
        LINE
      )}
    >
      {signature === null ? (
        <span className="text-muted-foreground">
          No client method: this route upgrades to a WebSocket.
        </span>
      ) : (
        <>
          {signature}
          {returns ? <span className="text-subtle">{`  →  ${returns}`}</span> : null}
        </>
      )}
    </p>
  );
}

export function SdkMethodEntry({
  resource,
  method,
  lang,
}: {
  resource: SdkResource;
  method: SdkMethod;
  lang: SdkLang;
}) {
  const sample = method.samples[lang];
  const body = bodyOf(method, lang);
  return (
    <section
      id={method.id}
      className={cn(
        "grid scroll-mt-[64px] gap-x-14 gap-y-7 border-t py-14 xl:grid-cols-[minmax(0,1fr)_minmax(0,500px)]",
        LINE
      )}
    >
      <div className="min-w-0">
        <h2 className="docs-count m-0 font-mono text-[22px]/[1.3] font-normal break-words text-fg">
          {methodName(method, lang)}
        </h2>
        <p className="mt-2 mb-0 text-[16px]/[1.6] text-fg/80">{method.summary}</p>
        <CallShape resource={resource} method={method} lang={lang} />
        {method.params.length > 0 ? (
          <div className="mt-7">
            <SdkFields title="Options">
              {method.params.map(field => (
                <SdkFieldRow
                  key={`${field.in ?? ""}${field.name}`}
                  name={field.name}
                  type={field.type}
                  required={field.required}
                  in={field.in}
                >
                  {field.description || null}
                </SdkFieldRow>
              ))}
            </SdkFields>
          </div>
        ) : null}
        {body ? (
          <p className="mt-6 mb-0 flex flex-wrap items-baseline gap-2.5 text-[14px] text-subtle">
            Request body
            <code className="font-mono text-[13.5px] break-all text-fg">{body}</code>
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
      <div className={cn("min-w-0 xl:sticky xl:self-start xl:pt-16", BELOW_HEADER)}>
        {sample ? (
          <div className="[&>div]:my-0">
            <SdkSample code={sample} lang={lang} label={lang === "go" ? "main.go" : "example.ts"} />
          </div>
        ) : null}
      </div>
    </section>
  );
}
