"use client";

/** One SDK resource: every method listed, then each with its call, its options and a sample. */
import { Button, SearchInput, cn } from "@compozy/ui";
import { useState } from "react";

import { LINE } from "@/components/docs/docs-classes";
import { useLinkedMethod } from "@/components/sdk/hooks/use-linked-method";
import { useSdkLang } from "@/components/sdk/hooks/use-sdk-lang";
import { SdkMethodEntry } from "@/components/sdk/sdk-method";
import {
  methodName,
  type SdkLang,
  type SdkMethod,
  type SdkResource,
} from "@/lib/sdk-reference-model";

const PREVIEW = 10;

function MethodIndex({ methods, lang }: { methods: readonly SdkMethod[]; lang: SdkLang }) {
  return (
    <ul className="not-prose m-0 mt-5 grid list-none gap-x-10 p-0 md:grid-cols-2 xl:grid-cols-3">
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

export function SdkResourceBody({ resource }: { resource: SdkResource }) {
  const lang = useSdkLang();
  const [query, setQuery] = useState("");
  const [all, setAll] = useState(false);
  const linked = useLinkedMethod(
    resource.ops.map(method => method.id),
    PREVIEW
  );
  const needle = query.trim().toLowerCase();
  const matched = needle
    ? resource.ops.filter(method =>
        `${method.id} ${methodName(method, lang)} ${method.summary}`.toLowerCase().includes(needle)
      )
    : resource.ops;
  const shown = all || linked || needle ? matched : matched.slice(0, PREVIEW);
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
          <SdkMethodEntry key={method.id} resource={resource} method={method} lang={lang} />
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
