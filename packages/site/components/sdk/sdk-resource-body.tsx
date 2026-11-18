"use client";

/** One SDK resource: every method listed, then each with its call, its options and a sample. */
import { SearchInput, buttonVariants, cn } from "@compozy/ui";
import { useState } from "react";

import { LINE } from "@/components/docs/docs-classes";
import { useLinkedMethod } from "@/components/sdk/hooks/use-linked-method";
import { useOpenDetails } from "@/components/sdk/hooks/use-open-details";
import { useSdkLang } from "@/components/sdk/hooks/use-sdk-lang";
import { SdkMethodEntry } from "@/components/sdk/sdk-method";
import {
  countOf,
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
  const expanded = all || linked || needle !== "";
  const more = useOpenDetails(expanded);
  const head = matched.slice(0, PREVIEW);
  const rest = matched.slice(PREVIEW);
  const entry = (method: SdkMethod) => (
    <SdkMethodEntry key={method.id} resource={resource} method={method} lang={lang} />
  );
  return (
    <>
      <div className="mt-8 flex flex-wrap items-center justify-between gap-4">
        <SearchInput
          value={query}
          onChange={setQuery}
          placeholder={`Filter ${countOf(resource.ops.length, "method")}`}
          aria-label="Filter methods"
          containerClassName="h-10 w-full sm:w-[320px] [&_input]:text-[14px]"
        />
        <p className="m-0 font-mono text-[12.5px] text-subtle tabular-nums">
          {matched.length} of {resource.ops.length}
        </p>
      </div>
      <MethodIndex methods={matched} lang={lang} />
      <div className="mt-12">{head.map(entry)}</div>
      {rest.length > 0 ? (
        // Every method is in the server HTML, so the index's anchors and crawlers reach them; past
        // the preview they wait in a closed `<details>`, which a browser opens for a link to one of
        // them (or find in page) even before the page hydrates.
        <details
          ref={more}
          // Chrome opens it for a URL fragment while the HTML loads, before React hydrates: `open`
          // is the browser's state here, not a server/client difference.
          suppressHydrationWarning
          onToggle={event => {
            if (event.currentTarget.open) setAll(true);
          }}
          className="group/more"
        >
          <summary
            className={cn(
              buttonVariants({ variant: "outline", size: "sm" }),
              "w-fit list-none group-open/more:hidden [&::-webkit-details-marker]:hidden"
            )}
          >
            Show all {countOf(matched.length, "method")}
          </summary>
          {rest.map(entry)}
        </details>
      ) : null}
    </>
  );
}
