"use client";

import type { LucideIcon } from "lucide-react";
import * as React from "react";

import { cn } from "../../lib/utils";
import { Eyebrow } from "./eyebrow";
import { HelpTip } from "./help-tip";

export interface FormSectionProps extends Omit<React.ComponentProps<"section">, "title"> {
  /** Section title, rendered at 12.5/600/-0.012em via the small-body token. */
  title: React.ReactNode;
  /** Optional leading icon, painted `--subtle` at 14 px with stroke 1.75. */
  icon?: LucideIcon;
  /** Optional right-aligned eyebrow at 11 px (counts, status, hints). */
  rightLabel?: React.ReactNode;
  /**
   * Explanatory prose for the section, moved into a `HelpTip` beside the title.
   * Use `description` instead when the sentence reports runtime truth.
   */
  help?: React.ReactNode;
  /**
   * Visible line between the head and the body. Reserved for what the runtime
   * will do — catalog state, inherited defaults, irreversibility. Explanation
   * belongs in `help`.
   */
  description?: React.ReactNode;
  children: React.ReactNode;
}

/**
 * One titled block of a modal body or settings form.
 *
 * A hairline rule and top padding separate sections, without adding a card
 * surface or horizontal padding. Fields therefore align with the dialog body
 * gutter; the first section drops the rule.
 */
function FormSection({
  title,
  icon: Icon,
  rightLabel,
  help,
  description,
  className,
  children,
  ...props
}: FormSectionProps) {
  return (
    <section
      data-slot="form-section"
      className={cn(
        "mt-7 flex flex-col border-t border-line pt-5.5 first:mt-0 first:border-t-0 first:pt-0",
        className
      )}
      {...props}
    >
      <header
        data-slot="form-section-head"
        className="mb-3.5 flex min-w-0 flex-wrap items-center gap-2"
      >
        {Icon ? (
          <span
            aria-hidden="true"
            data-slot="form-section-icon"
            className="inline-flex size-3.5 shrink-0 items-center justify-center text-subtle"
          >
            <Icon width={14} height={14} strokeWidth={1.75} />
          </span>
        ) : null}
        <h3
          data-slot="form-section-title"
          className="min-w-0 truncate text-small-body font-semibold tracking-modal-title text-fg-strong"
        >
          {title}
        </h3>
        {help ? <HelpTip label={`About ${sectionHelpLabel(title)}`}>{help}</HelpTip> : null}
        {rightLabel ? (
          <Eyebrow
            data-slot="form-section-right-label"
            className="ml-auto min-w-0 truncate text-muted"
          >
            {rightLabel}
          </Eyebrow>
        ) : null}
      </header>
      {description ? (
        <p
          data-slot="form-section-description"
          className="mb-3 text-form-hint leading-normal text-muted"
        >
          {description}
        </p>
      ) : null}
      <div data-slot="form-section-body" className="flex min-w-0 flex-col gap-4.5 [&>*+*]:mt-0">
        {children}
      </div>
    </section>
  );
}

/** Names the help trigger after its section when the title is plain text. */
function sectionHelpLabel(title: React.ReactNode) {
  return typeof title === "string" ? title.toLowerCase().replace(/[?.]$/, "") : "this section";
}

export { FormSection };
