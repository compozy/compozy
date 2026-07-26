import { Section } from "@agh/ui";

import type { LoopInputSchema } from "../../types";
import { MonoTag } from "../mono-tag";

interface LoopDeclaredInputsProps {
  inputs?: LoopInputSchema;
}

function formatDefault(value: unknown): string | null {
  if (value === undefined || value === null) return null;
  if (typeof value === "string") return value === "" ? '"" (empty)' : value;
  if (typeof value === "boolean" || typeof value === "number") return String(value);
  return JSON.stringify(value);
}

/** Right-rail panel: the Loop's declared inputs (name, required, type, default). */
export function LoopDeclaredInputs({ inputs }: LoopDeclaredInputsProps) {
  const names = inputs ? Object.keys(inputs) : [];
  return (
    <Section label="Declared inputs" data-testid="loop-declared-inputs">
      <div className="flex flex-col rounded-lg border border-line bg-canvas-soft">
        {names.length === 0 ? (
          <p className="px-3.5 py-3 text-form-hint text-subtle">This Loop declares no inputs.</p>
        ) : (
          names.map(name => {
            const field = inputs?.[name];
            const defaultLabel = formatDefault(field?.default);
            return (
              <div key={name} className="border-t border-line-soft px-3.5 py-2.5 first:border-t-0">
                <div className="flex items-center gap-1.5">
                  <span className="font-mono text-xs text-fg-strong">{name}</span>
                  {field?.required ? (
                    <span className="font-semibold text-muted" aria-label="required">
                      *
                    </span>
                  ) : null}
                  <MonoTag className="ml-auto rounded-xs bg-badge-fill px-1.5 py-0.5">
                    {field?.type}
                  </MonoTag>
                </div>
                {field?.description ? (
                  <p className="mt-1 text-form-hint leading-snug text-subtle">
                    {field.description}
                  </p>
                ) : null}
                {defaultLabel ? (
                  <p className="mt-1 font-mono text-mono-id text-faint">default: {defaultLabel}</p>
                ) : null}
              </div>
            );
          })
        )}
      </div>
    </Section>
  );
}
