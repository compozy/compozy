import { TextCursorInput } from "lucide-react";

import type { LoopInputSchema } from "../../types";
import { LoopRailSection } from "../loop-rail-section";
import { loopInputLabel } from "../../lib/loop-run-form";

interface LoopDeclaredInputsProps {
  inputs?: LoopInputSchema;
}

function formatDefault(value: unknown): string | null {
  if (value === undefined || value === null) return null;
  if (typeof value === "string") return value === "" ? "empty" : value;
  if (typeof value === "boolean" || typeof value === "number") return String(value);
  return JSON.stringify(value);
}

/**
 * Right-rail panel: the Loop's declared inputs in plain words. The input key stays
 * one hover away (`title`); the control on the run form already implies the type.
 */
export function LoopDeclaredInputs({ inputs }: LoopDeclaredInputsProps) {
  const names = inputs ? Object.keys(inputs) : [];
  const required = names.filter(name => inputs?.[name]?.required).length;
  return (
    <LoopRailSection
      data-testid="loop-declared-inputs"
      defaultOpen
      gist={`${required} required · ${names.length - required} optional`}
      icon={<TextCursorInput aria-hidden="true" className="size-3.5" />}
      title="Inputs"
    >
      <div className="flex flex-col">
        {names.length === 0 ? (
          <p className="px-4 py-3 text-form-hint text-subtle">No inputs needed.</p>
        ) : (
          names.map(name => {
            const field = inputs?.[name];
            const defaultLabel = formatDefault(field?.default);
            return (
              <div key={name} className="border-t border-line-soft px-4 py-2.5 first:border-t-0">
                <div className="flex items-center gap-1.5">
                  <span className="text-form-label text-fg-strong" title={name}>
                    {loopInputLabel(name, field)}
                  </span>
                  {field?.required ? (
                    <span className="font-semibold text-muted" aria-label="required">
                      *
                    </span>
                  ) : null}
                </div>
                {field?.description ? (
                  <p className="mt-1 text-form-hint leading-snug text-subtle">
                    {field.description}
                  </p>
                ) : null}
                {defaultLabel ? (
                  <p className="mt-1 text-form-hint text-subtle">Default: {defaultLabel}</p>
                ) : null}
              </div>
            );
          })
        )}
      </div>
    </LoopRailSection>
  );
}
