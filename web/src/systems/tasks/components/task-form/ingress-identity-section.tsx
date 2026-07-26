import { Field, FieldLabel, Input } from "@agh/ui";

interface IngressIdentitySectionProps {
  identifier: string;
  onIdentifier: (value: string) => void;
}

/** Ingress & identity — optional stable identifier override, unbound by default. */
export function IngressIdentitySection({ identifier, onIdentifier }: IngressIdentitySectionProps) {
  return (
    <Field>
      <FieldLabel htmlFor="task-identifier-input">
        Identifier
        <span className="font-normal text-faint"> (override)</span>
      </FieldLabel>
      <Input
        className="font-mono"
        data-testid="task-identifier-input"
        id="task-identifier-input"
        onChange={event => onIdentifier(event.target.value)}
        placeholder="TASK-123"
        value={identifier}
      />
    </Field>
  );
}
