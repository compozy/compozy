"use client";

/** The reading column with the section rail beside it; the rail's beam fills as the body is read. */
import { cn } from "@compozy/ui";
import { m, useReducedMotion, useScroll } from "motion/react";
import { useRef, type ReactNode, type RefObject } from "react";

import { Small } from "@/components/site/primitives";

import { useActiveHeading } from "./hooks/use-active-heading";

export interface RailSection {
  id: string;
  text: string;
}

function SectionRail({
  sections,
  body,
}: {
  sections: readonly RailSection[];
  body: RefObject<HTMLDivElement | null>;
}) {
  const active = useActiveHeading(sections.map(section => section.id));
  const reduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: body, offset: ["start 35%", "end 65%"] });
  return (
    <nav
      aria-label="On this page"
      className="hidden lg:sticky lg:top-28 lg:block lg:self-start lg:pl-12"
    >
      <Small className="mb-4">On this page</Small>
      <div className="relative">
        <span aria-hidden className="absolute inset-y-0 left-0 w-0.5 bg-site-light-120">
          <m.span
            style={{ scaleY: reduced ? 1 : scrollYProgress }}
            className="absolute inset-0 origin-top bg-[linear-gradient(180deg,#1df0d7,#1f87fc)]"
          />
        </span>
        <ol className="m-0 list-none p-0">
          {sections.map((section, order) => (
            <li key={section.id}>
              <a
                href={`#${section.id}`}
                aria-current={active === section.id ? "location" : undefined}
                className={cn(
                  "flex gap-3 py-2 pl-5 text-[14px]/[1.4] transition-colors",
                  active === section.id ? "text-fg" : "text-muted-foreground hover:text-fg"
                )}
              >
                <span className="font-mono text-[12px]/[1.65] text-subtle tabular-nums">
                  {String(order + 1).padStart(2, "0")}
                </span>
                {section.text}
              </a>
            </li>
          ))}
        </ol>
      </div>
    </nav>
  );
}

export function ReadingLayout({
  sections,
  children,
}: {
  sections: readonly RailSection[];
  children: ReactNode;
}) {
  const body = useRef<HTMLDivElement>(null);
  return (
    <div className="mt-16 grid lg:grid-cols-[1fr_minmax(0,720px)_1fr]">
      <div ref={body} className="blog-prose min-w-0 lg:col-start-2">
        {children}
      </div>
      <SectionRail sections={sections} body={body} />
    </div>
  );
}
