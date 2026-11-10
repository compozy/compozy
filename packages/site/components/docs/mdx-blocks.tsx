/** The docs' own MDX components, redrawn: notes, callouts, link lists and workflows. */
import { cn } from "@compozy/ui";
import { ArrowRight, ArrowUpRight, Info, TriangleAlert } from "@compozy/ui/icons";
import Link from "next/link";
import type { ReactNode } from "react";

import { BEAM, FOCUS, LINE } from "@/components/docs/docs-classes";

const INSET =
  "text-[15px] leading-[1.65] [&_p]:my-2 [&_p:first-child]:mt-0 [&_p:last-child]:mb-0 [&>:first-child]:mt-0 [&>:last-child]:mb-0";

interface LinkItemProps {
  href: string;
  label: string;
  title: string;
  description: string;
  meta?: string;
}

export function OperatorNote({
  label = "Note",
  children,
}: {
  label?: string;
  children: ReactNode;
}) {
  return (
    <aside
      role="note"
      className="not-prose relative my-8 rounded-[14px] border border-site-light-090 bg-site-light-025 py-5 pr-6 pl-7"
    >
      <span aria-hidden className={cn("absolute inset-y-5 left-0 w-0.5 rounded-full", BEAM)} />
      <p className="m-0 mb-2 font-mono text-[12.5px] text-brand-strong">{label}</p>
      <div className={INSET}>{children}</div>
    </aside>
  );
}

const TONES: Record<string, string> = {
  warn: "border-warning/40 [&>svg]:text-warning",
  warning: "border-warning/40 [&>svg]:text-warning",
  error: "border-danger/45 [&>svg]:text-danger",
};

export function Callout({
  type = "info",
  title,
  children,
}: {
  type?: string;
  title?: ReactNode;
  children: ReactNode;
}) {
  const Glyph = type === "info" ? Info : TriangleAlert;
  return (
    <aside
      role="note"
      className={cn(
        "not-prose my-8 flex gap-3.5 rounded-[14px] border bg-site-light-025 p-5",
        TONES[type] ?? "border-brand/40 [&>svg]:text-brand-strong"
      )}
    >
      <Glyph className="mt-1 size-4 shrink-0" aria-hidden />
      <div className={cn(INSET, "min-w-0")}>
        {title ? <p className="font-medium text-fg">{title}</p> : null}
        {children}
      </div>
    </aside>
  );
}

export function RouteList({ children }: { children: ReactNode }) {
  return (
    <ul
      className={cn(
        "not-prose my-8 list-none! divide-y divide-site-light-080 border-y p-0 pl-0!",
        LINE
      )}
    >
      {children}
    </ul>
  );
}

export function RouteRow({ href, label, title, description, meta }: LinkItemProps) {
  return (
    <li className="my-0!">
      <Link
        href={href}
        className={cn(
          "group grid w-full gap-x-6 gap-y-1 py-5 text-left sm:grid-cols-[120px_minmax(0,1fr)_auto]",
          FOCUS
        )}
      >
        <span className="pt-0.5 font-mono text-[12.5px] text-subtle">{label}</span>
        <span className="min-w-0">
          <span className="block text-[16.5px]/[1.35] text-fg">{title}</span>
          <span className="mt-1 block text-[14.5px]/[1.55] text-muted-foreground">
            {description}
          </span>
        </span>
        <span className="flex items-center gap-3 self-start pt-0.5 text-[13px] text-subtle">
          {meta}
          <ArrowRight
            className="size-4 transition-transform group-hover:translate-x-1 group-hover:text-fg"
            aria-hidden
          />
        </span>
      </Link>
    </li>
  );
}

export function GuideGrid({ children }: { children: ReactNode }) {
  return <div className="not-prose my-8 grid gap-4 sm:grid-cols-2">{children}</div>;
}

export function GuideCard({ href, label, title, description, meta }: LinkItemProps) {
  return (
    <Link
      href={href}
      className={cn(
        "group flex flex-col rounded-[16px] border border-site-light-090 bg-site-light-025 p-5 text-left transition-colors hover:border-site-light-200 hover:bg-site-light-040",
        FOCUS
      )}
    >
      <span className="flex items-center justify-between font-mono text-[12.5px] text-subtle">
        {label || meta}
        <ArrowUpRight
          className="size-4 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5 group-hover:text-fg"
          aria-hidden
        />
      </span>
      <span className="mt-4 block text-[17px]/[1.3] text-fg">{title}</span>
      <span className="mt-2 block text-[14.5px]/[1.55] text-muted-foreground">{description}</span>
      {label && meta ? <span className="mt-4 text-[13px] text-subtle">{meta}</span> : null}
    </Link>
  );
}

export function Workflow({ children }: { children: ReactNode }) {
  return (
    <ol className="docs-steps not-prose relative my-9 list-none! p-0 pl-0!">
      <span aria-hidden className="absolute top-3 bottom-3 left-[13px] w-px bg-site-light-120" />
      {children}
    </ol>
  );
}

export function WorkflowStep({ title, children }: { title: string; children: ReactNode }) {
  return (
    <li className="relative my-0! grid grid-cols-[28px_minmax(0,1fr)] gap-5 pb-8 last:pb-0">
      <span
        aria-hidden
        className="docs-step-number relative z-10 inline-flex size-7 items-center justify-center rounded-full border border-site-light-200 bg-[#0a0a0a] font-mono text-[12px] text-brand-strong tabular-nums"
      />
      <div className="min-w-0 pt-0.5">
        <p className="m-0 text-[16.5px] font-medium text-fg">{title}</p>
        <div className={cn(INSET, "mt-2 text-fg/75")}>{children}</div>
      </div>
    </li>
  );
}
