import Link from "next/link";
import { ArrowRight, CheckCircle2 } from "lucide-react";
import type { ReactNode } from "react";
import { Eyebrow } from "@agh/ui";

interface RouteRowProps {
  href: string;
  label: string;
  title: string;
  description: string;
  meta?: string;
}

interface GuideCardProps {
  href: string;
  label: string;
  title: string;
  description: string;
  meta?: string;
}

interface WorkflowStepProps {
  title: string;
  children: ReactNode;
}

export function OperatorNote({
  label = "Operator note",
  children,
}: {
  label?: string;
  children: ReactNode;
}) {
  return (
    <aside
      role="note"
      className="not-prose rounded-xl border border-line bg-canvas-soft p-5 md:px-6"
    >
      <Eyebrow className="text-accent">{label}</Eyebrow>
      <div className="mt-3 text-base leading-7 text-muted">{children}</div>
    </aside>
  );
}

export function RouteList({ children }: { children: ReactNode }) {
  return (
    <div className="not-prose overflow-hidden rounded-xl border border-line bg-canvas-soft">
      {children}
    </div>
  );
}

export function RouteRow({ href, label, title, description, meta }: RouteRowProps) {
  return (
    <Link
      href={href}
      className="group grid gap-3 border-t border-line p-5 transition-colors first:border-t-0 hover:bg-hover md:grid-cols-[132px_minmax(0,1fr)_150px] md:items-center md:px-6"
    >
      <Eyebrow className="text-subtle">{label}</Eyebrow>

      <div className="min-w-0">
        <p className="text-lg font-semibold tracking-tight text-fg">{title}</p>
        <p className="mt-1 text-sm leading-6 text-muted">{description}</p>
      </div>

      <div className="flex items-center gap-2 text-sm text-muted md:justify-end">
        {meta ? (
          <span className="hidden md:inline">{meta}</span>
        ) : (
          <span className="hidden md:inline">Open section</span>
        )}
        <ArrowRight
          aria-hidden
          className="size-4 text-accent transition-transform group-hover:translate-x-0.5"
        />
      </div>
    </Link>
  );
}

export function GuideGrid({ children }: { children: ReactNode }) {
  return <div className="not-prose my-8 grid gap-4 md:grid-cols-2">{children}</div>;
}

export function GuideCard({ href, label, title, description, meta }: GuideCardProps) {
  return (
    <Link
      href={href}
      className="group flex min-h-55 flex-col justify-between rounded-xl border border-line bg-canvas-soft p-5 transition-colors hover:border-accent/40 md:p-6"
    >
      <div>
        <div className="flex flex-wrap items-center gap-2">
          <Eyebrow className="text-accent">{label}</Eyebrow>
          {meta ? (
            <Eyebrow className="rounded-md border border-line bg-elevated px-2 py-1 text-subtle">
              {meta}
            </Eyebrow>
          ) : null}
        </div>

        <p className="mt-4 text-xl leading-7 font-semibold tracking-tight text-fg">{title}</p>
        <p className="mt-3 text-sm leading-6 text-muted">{description}</p>
      </div>

      <div className="mt-6 flex items-center gap-2 text-sm text-muted">
        <span>Open guide</span>
        <ArrowRight
          aria-hidden
          className="size-4 text-accent transition-transform group-hover:translate-x-0.5"
        />
      </div>
    </Link>
  );
}

export function Workflow({ children }: { children: ReactNode }) {
  return (
    <div className="not-prose my-8 overflow-hidden rounded-xl border border-line bg-canvas-soft">
      {children}
    </div>
  );
}

export function WorkflowStep({ title, children }: WorkflowStepProps) {
  return (
    <section className="grid min-w-0 gap-4 border-t border-line p-5 first:border-t-0 md:grid-cols-[40px_minmax(0,1fr)] md:px-6">
      <div className="flex size-10 items-center justify-center rounded-lg border border-line bg-elevated">
        <CheckCircle2 className="size-4 text-accent" />
      </div>
      <div className="min-w-0">
        <p className="text-lg leading-7 font-semibold tracking-tight text-fg">{title}</p>
        <div className="mt-2 text-sm leading-7 text-muted">{children}</div>
      </div>
    </section>
  );
}
