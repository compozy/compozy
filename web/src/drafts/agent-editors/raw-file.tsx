/** The one quiet disclosure: the file behind the page, for people who want it. */
import type { ReactNode } from "react";

import { Disclosure, Field, FieldError, Textarea } from "@compozy/ui";

export function RawFile({
  value,
  error,
  defaultOpen = false,
  note,
  onChange,
}: {
  value: string;
  error?: string;
  defaultOpen?: boolean;
  note?: ReactNode;
  onChange: (next: string) => void;
}) {
  return (
    <Disclosure defaultOpen={defaultOpen} label="Advanced" size="md">
      <Field data-invalid={error ? true : undefined}>
        <Textarea
          aria-invalid={error ? true : undefined}
          aria-label="The file behind this page"
          className="min-h-72 font-mono text-code-block"
          onChange={event => onChange(event.target.value)}
          value={value}
        />
        {error ? <FieldError>{error}</FieldError> : null}
        {note ? <span className="text-eyebrow text-subtle">{note}</span> : null}
      </Field>
    </Disclosure>
  );
}
