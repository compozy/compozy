import { Input } from "@agh/ui";

import type { LoopInputSchema } from "../../types";

interface LoopInputMappingProps {
  inputs: LoopInputSchema;
  mapping: Record<string, string>;
  onChange: (key: string, path: string) => void;
}

/**
 * Event-payload -> input mapping table (triggers/webhooks only). Each declared
 * input gets a path field (`trigger.payload.slug`) resolved from the activation
 * envelope at fire time. A mapped input overrides its static value.
 */
export function LoopInputMapping({ inputs, mapping, onChange }: LoopInputMappingProps) {
  const names = Object.keys(inputs);
  if (names.length === 0) return null;
  return (
    <div className="flex flex-col gap-2" data-testid="loop-input-mapping">
      <p className="text-form-hint text-subtle">
        Map fields from the event payload into the Loop inputs. Leave a row blank to use the static
        value above.
      </p>
      <div className="flex flex-col rounded-md border border-line-soft">
        {names.map(name => (
          <div
            key={name}
            className="grid grid-cols-[minmax(0,140px)_minmax(0,1fr)] items-center gap-2 border-t border-line-soft px-3 py-2 first:border-t-0"
            data-testid="loop-mapping-row"
            data-input={name}
          >
            <span className="truncate font-mono text-mono-id text-fg-strong">{name}</span>
            <Input
              className="font-mono"
              data-testid={`loop-mapping-field-${name}`}
              placeholder="trigger.payload.field"
              value={mapping[name] ?? ""}
              onChange={event => onChange(name, event.target.value)}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
