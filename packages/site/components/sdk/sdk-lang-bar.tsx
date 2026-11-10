"use client";

/** Stays under the header while you read, so the language is always one click away. */
import { cn } from "@compozy/ui";

import { BELOW_HEADER, FOCUS, LINE } from "@/components/docs/docs-classes";
import { setSdkLang, useSdkLang } from "@/components/sdk/hooks/use-sdk-lang";
import { SDK_LANGS } from "@/lib/sdk-reference-model";

const SEGMENT = cn(
  "inline-flex h-8 cursor-pointer items-center rounded-[9px] bg-transparent px-3.5 text-[13.5px] transition-colors",
  FOCUS
);

export function SdkLangBar() {
  const lang = useSdkLang();
  const current = SDK_LANGS.find(item => item.id === lang) ?? SDK_LANGS[0];
  return (
    <div
      className={cn(
        "not-prose sticky z-20 -mx-5 flex flex-wrap items-center justify-between gap-x-6 gap-y-2 border-b bg-[#0a0a0a]/90 px-5 py-3 backdrop-blur-xl md:-mx-10 md:px-10",
        BELOW_HEADER,
        LINE
      )}
    >
      <code className="min-w-0 truncate font-mono text-[13px] text-muted-foreground">
        {current?.module}
      </code>
      <div
        role="group"
        aria-label="Language"
        className="inline-flex gap-1 rounded-[12px] border border-site-light-100 p-1"
      >
        {SDK_LANGS.map(item => (
          <button
            key={item.id}
            type="button"
            aria-pressed={lang === item.id}
            onClick={() => setSdkLang(item.id)}
            className={cn(
              SEGMENT,
              lang === item.id ? "bg-site-light-100 text-fg" : "text-muted-foreground hover:text-fg"
            )}
          >
            {item.label}
          </button>
        ))}
      </div>
    </div>
  );
}
