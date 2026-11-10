/** A CLI command page: the generated help text, laid out as a signature, examples and a flag table. */
import { CopyIconButton, cn } from "@compozy/ui";
import { ArrowRight } from "@compozy/ui/icons";
import Link from "next/link";
import type { ComponentProps } from "react";
import Markdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

import { DOCS_PROSE, FOCUS, LINE, textOf } from "@/components/docs/docs-classes";
import {
  DocsCode,
  DocsH2,
  DocsH3,
  DocsInlineCode,
  DocsLink,
  DocsPre,
  DocsTable,
} from "@/components/docs/docs-prose";
import {
  headingSlug,
  type CliCommand,
  type CliDoc,
  type CliExample,
  type CliFlag,
  type CliSection,
} from "@/lib/cli-doc";

const HEADING =
  "mt-14 mb-5 flex items-baseline gap-3 font-display text-[27px]/[1.2] font-normal tracking-[-0.02em] text-fg";

function Signature({ usage }: { usage: string }) {
  const [command, ...rest] = usage.split(/ (?=[<[])/);
  return (
    <div
      className={cn(
        "flex items-center gap-3 rounded-[16px] border bg-[#0d0d0d] py-4 pr-3 pl-5 font-mono text-[15px]/[1.6]",
        LINE
      )}
    >
      <span aria-hidden className="text-brand-strong">
        $
      </span>
      <code className="min-w-0 flex-1 break-words text-fg">
        {command} <span className="text-muted-foreground">{rest.join(" ")}</span>
      </code>
      <CopyIconButton value={usage} />
    </div>
  );
}

function Examples({ examples }: { examples: readonly CliExample[] }) {
  return (
    <ol className="m-0 list-none p-0">
      {examples.map(example => (
        <li key={example.code} className={cn("border-t py-6 first:border-t-0 first:pt-0", LINE)}>
          {example.note ? (
            <p className="m-0 text-[15.5px]/[1.5] text-fg/85">{example.note}</p>
          ) : null}
          <div className="[&>div]:mt-3 [&>div]:mb-0">
            <DocsCode code={example.code} language="bash" />
          </div>
        </li>
      ))}
    </ol>
  );
}

function FlagTable({ flags }: { flags: readonly CliFlag[] }) {
  return (
    <dl className={cn("m-0 divide-y divide-site-light-080 border-y", LINE)}>
      {flags.map(flag => (
        <div
          key={flag.long}
          className="grid gap-x-8 gap-y-1.5 py-4 sm:grid-cols-[260px_minmax(0,1fr)]"
        >
          <dt className="flex flex-wrap items-center gap-2 font-mono text-[13.5px]">
            <span className="text-fg">{flag.long}</span>
            {flag.short ? <span className="text-subtle">{flag.short}</span> : null}
            {flag.type ? (
              <span className="rounded-[6px] border border-site-light-100 px-1.5 py-px text-[11.5px] text-muted-foreground">
                {flag.type}
              </span>
            ) : null}
          </dt>
          <dd className="m-0 text-[14.5px]/[1.55] break-words text-fg/75">{flag.text}</dd>
        </div>
      ))}
    </dl>
  );
}

function Subcommands({ commands }: { commands: readonly CliCommand[] }) {
  return (
    <ul className="m-0 grid list-none gap-x-10 p-0 md:grid-cols-2">
      {commands.map(command => (
        <li key={command.path} className={cn("border-b", LINE)}>
          <Link
            href={`/docs/${command.path}`}
            className={cn("group flex w-full items-start gap-3 py-3.5 text-left", FOCUS)}
          >
            <span className="min-w-0 flex-1">
              <span className="block font-mono text-[13.5px] text-fg">
                {command.name.split(" ").at(-1)}
              </span>
              <span className="mt-1 block text-[13.5px]/[1.5] text-muted-foreground">
                {command.text}
              </span>
            </span>
            <ArrowRight
              className="mt-0.5 size-3.5 shrink-0 text-subtle transition-transform group-hover:translate-x-0.5 group-hover:text-fg"
              aria-hidden
            />
          </Link>
        </li>
      ))}
    </ul>
  );
}

function SectionTitle({ title, count }: CliSection) {
  return (
    <h2 id={headingSlug(title)} className={HEADING}>
      {title}
      {title === "Usage" ? null : (
        <span className="docs-count font-mono text-[13px] text-subtle tabular-nums">{count}</span>
      )}
    </h2>
  );
}

function RestH2({ children }: ComponentProps<"h2">) {
  return <DocsH2 id={headingSlug(textOf(children))}>{children}</DocsH2>;
}

function RestH3({ children }: ComponentProps<"h3">) {
  return <DocsH3 id={headingSlug(textOf(children))}>{children}</DocsH3>;
}

/** The remainder renders through the same prose components as every MDX page. */
const REST_COMPONENTS: Components = {
  h2: RestH2,
  h3: RestH3,
  a: DocsLink,
  code: DocsInlineCode,
  pre: DocsPre,
  table: DocsTable,
};

export function CliBody({ cli }: { cli: CliDoc }) {
  return (
    <div className="[&>h2:first-child]:mt-0">
      {cli.usage ? (
        <>
          <SectionTitle title="Usage" count={1} />
          <Signature usage={cli.usage} />
        </>
      ) : null}
      {cli.subcommands.length > 0 ? (
        <>
          <SectionTitle title="Subcommands" count={cli.subcommands.length} />
          <Subcommands commands={cli.subcommands} />
        </>
      ) : null}
      {cli.examples.length > 0 ? (
        <>
          <SectionTitle title="Examples" count={cli.examples.length} />
          <Examples examples={cli.examples} />
        </>
      ) : null}
      {cli.flags.length > 0 ? (
        <>
          <SectionTitle title="Options" count={cli.flags.length} />
          <FlagTable flags={cli.flags} />
        </>
      ) : null}
      {cli.inherited.length > 0 ? (
        <>
          <SectionTitle title="Inherited options" count={cli.inherited.length} />
          <FlagTable flags={cli.inherited} />
        </>
      ) : null}
      {cli.rest ? (
        <div className={cn("mt-14", DOCS_PROSE)}>
          <Markdown remarkPlugins={[remarkGfm]} components={REST_COMPONENTS}>
            {cli.rest}
          </Markdown>
        </div>
      ) : null}
    </div>
  );
}
