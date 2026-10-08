import type { ComponentProps, ReactNode } from "react";

import { FormSection } from "@compozy/ui";

interface AutomationFormSectionProps extends Omit<ComponentProps<typeof FormSection>, "title"> {
  /** Position in the form; Only if appears for events and links and renumbers Does. */
  number: number;
  title: ReactNode;
}

/** One numbered editor section: `02 Starts` with its question underneath. */
export function AutomationFormSection({ number, title, ...props }: AutomationFormSectionProps) {
  return (
    <FormSection
      title={
        <>
          <span className="mr-2 font-mono text-faint tabular-nums">
            {String(number).padStart(2, "0")}
          </span>
          {title}
        </>
      }
      {...props}
    />
  );
}
