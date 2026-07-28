"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export interface ImmutableIdentityRow {
  /** Stable key and visible label for the locked field. */
  label: string;
  value: React.ReactNode;
  /** Renders the value in the mono stack — use for refs, paths, and ids. */
  mono?: boolean;
}

export interface ImmutableIdentityProps extends Omit<React.ComponentProps<"dl">, "children"> {
  rows: ReadonlyArray<ImmutableIdentityRow>;
  /** Why these fields cannot change, e.g. the contract that omits them. */
  hint?: React.ReactNode;
}

/**
 * Readable summary of fields an update contract cannot mutate.
 *
 * A disabled input is forbidden as a data display (`MODAL-STANDARD.md`
 * § Component grammar): it reads as a control that happens to be off rather
 * than as a value that cannot change, and it is skipped by keyboard traversal
 * while still being announced as a form field.
 */
function ImmutableIdentity({ rows, hint, className, ...props }: ImmutableIdentityProps) {
  return (
    <div className="flex flex-col gap-2" data-slot="immutable-identity">
      <dl
        className={cn("flex flex-col gap-2 rounded-md bg-canvas px-3 py-2.5", className)}
        data-slot="immutable-identity-rows"
        {...props}
      >
        {rows.map(row => (
          <div className="flex min-w-0 items-baseline gap-3" key={row.label}>
            <dt className="w-kv-label shrink-0 text-form-label text-muted">{row.label}</dt>
            <dd
              className={cn(
                "min-w-0 flex-1 break-words text-form-input text-fg",
                row.mono && "font-mono text-mono-id tracking-normal"
              )}
            >
              {row.value}
            </dd>
          </div>
        ))}
      </dl>
      {hint ? (
        <p className="text-form-hint text-subtle" data-slot="immutable-identity-hint">
          {hint}
        </p>
      ) : null}
    </div>
  );
}

export { ImmutableIdentity };
