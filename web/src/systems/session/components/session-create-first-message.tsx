import { Field, FieldLabel, Textarea } from "@compozy/ui";

interface SessionCreateFirstMessageProps {
  value: string;
  onChange: (next: string) => void;
  disabled: boolean;
}

/**
 * What the session should do, written before it exists.
 *
 * Leaving it blank starts the session and nothing more; filling it in sends
 * once the session — and the environment it was bound to — is ready.
 */
function SessionCreateFirstMessage({ value, onChange, disabled }: SessionCreateFirstMessageProps) {
  return (
    <Field>
      <FieldLabel htmlFor="session-create-first-message">First message</FieldLabel>
      <Textarea
        data-testid="session-create-composer"
        disabled={disabled}
        id="session-create-first-message"
        onChange={event => onChange(event.target.value)}
        placeholder="Investigate the checkout latency regression"
        rows={3}
        value={value}
      />
    </Field>
  );
}

export { SessionCreateFirstMessage };
