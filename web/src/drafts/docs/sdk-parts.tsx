/** Shared pieces of the SDK reference: the language switch, field lists and signatures. */
import { cn } from "@compozy/ui";
import { use } from "react";

import { DocsCode } from "./prose";
import { LANGS, type SdkField, type SdkLang } from "./sdk";
import { SdkLangContext, SdkSetLangContext } from "./sdk-context";
import { BELOW_HEADER, FOCUS, LINE } from "./text";

const SEGMENT = cn(
  "inline-flex h-8 cursor-pointer items-center rounded-[9px] bg-transparent px-3.5 text-[13.5px] transition-colors",
  FOCUS
);

/** Stays under the header while you read, so the language is always one click away. */
export function SdkLangBar() {
  const lang = use(SdkLangContext);
  const setLang = use(SdkSetLangContext);
  const current = LANGS.find(item => item.id === lang) ?? LANGS[0];
  return (
    <div
      className={cn(
        "sticky z-20 -mx-5 flex flex-wrap items-center justify-between gap-x-6 gap-y-2 border-b bg-[#0a0a0a]/90 px-5 py-3 backdrop-blur-xl md:-mx-10 md:px-10",
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
        className="inline-flex gap-1 rounded-[12px] border border-[color:var(--light-100)] p-1"
      >
        {LANGS.map(item => (
          <button
            key={item.id}
            type="button"
            aria-pressed={lang === item.id}
            onClick={() => setLang(item.id)}
            className={cn(
              SEGMENT,
              lang === item.id
                ? "bg-[color:var(--light-100)] text-fg"
                : "text-muted-foreground hover:text-fg"
            )}
          >
            {item.label}
          </button>
        ))}
      </div>
    </div>
  );
}

export function SdkFields({ title, fields }: { title: string; fields: readonly SdkField[] }) {
  if (fields.length === 0) return null;
  return (
    <div className="mt-7">
      <p className="m-0 mb-1 text-[14px] text-subtle">{title}</p>
      <dl className={cn("m-0 divide-y divide-[color:var(--light-080)] border-y", LINE)}>
        {fields.map(field => (
          <div key={`${field.in ?? ""}${field.name}`} className="py-3.5">
            <dt className="flex flex-wrap items-center gap-2.5 font-mono text-[13.5px]">
              <span className="text-fg">{field.name}</span>
              <span className="text-subtle">{field.type}</span>
              {field.in ? (
                <span className="rounded-[6px] border border-[color:var(--light-100)] px-1.5 py-px text-[11.5px] text-muted-foreground">
                  {field.in}
                </span>
              ) : null}
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

export function SdkSignature({ code, lang }: { code: string; lang: SdkLang }) {
  return (
    <div className="[&>div]:my-0">
      <DocsCode code={code} language={lang === "go" ? "go" : "ts"} label="Signature" />
    </div>
  );
}

export function SdkSample({ code, lang, label }: { code: string; lang: SdkLang; label: string }) {
  return (
    <div className="[&>div]:my-0">
      <DocsCode code={code} language={lang === "go" ? "go" : "ts"} label={label} />
    </div>
  );
}
