/**
 * One symbol of a client page, per language: its signature, its options or fields, and an example.
 * Authored in MDX; the signature and example are fenced code, so Shiki highlights them at build.
 */
import { cn } from "@compozy/ui";
import type { ReactNode } from "react";

import { LINE } from "@/components/docs/docs-classes";
import { SdkLang } from "@/components/sdk/sdk-lang";
import { SDK_NO_GO_EQUIVALENT, type SdkLang as Lang } from "@/lib/sdk-reference-model";

export function SdkSymbol({
  lang,
  none = false,
  children,
}: {
  lang: Lang;
  /** The other client has no equivalent: say so instead of showing nothing. */
  none?: boolean;
  children?: ReactNode;
}) {
  return (
    <SdkLang lang={lang}>
      {none ? (
        <p className="not-prose my-0 rounded-[12px] border border-site-light-090 px-4 py-3 text-[14.5px] text-muted-foreground">
          {SDK_NO_GO_EQUIVALENT}
        </p>
      ) : (
        <div className="flex flex-col gap-7 [&>*]:my-0!">{children}</div>
      )}
    </SdkLang>
  );
}

export function SdkFields({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="not-prose">
      <p className="m-0 mb-1 text-[14px] text-subtle">{title}</p>
      <dl className={cn("m-0 divide-y divide-site-light-080 border-y", LINE)}>{children}</dl>
    </div>
  );
}

export function SdkFieldRow(props: {
  name: string;
  type: string;
  required?: boolean;
  /** Where an HTTP option goes: `path`, `query` or `header`. */
  in?: string;
  children?: ReactNode;
}) {
  return (
    <div className="py-3.5">
      <dt className="flex flex-wrap items-center gap-2.5 font-mono text-[13.5px]">
        <span className="text-fg">{props.name}</span>
        <span className="break-all text-subtle">{props.type}</span>
        {props.in ? (
          <span className="rounded-[6px] border border-site-light-100 px-1.5 py-px text-[11.5px] text-muted-foreground">
            {props.in}
          </span>
        ) : null}
        {props.required ? <span className="text-[12px] text-warning">required</span> : null}
      </dt>
      {props.children ? (
        <dd className="m-0 mt-1.5 text-[14.5px]/[1.55] text-fg/75 [&_code]:text-[0.88em]">
          {props.children}
        </dd>
      ) : null}
    </div>
  );
}
