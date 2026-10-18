import { Button, Field, FieldLabel, OwnerAvatar } from "@compozy/ui";

export interface SessionForkFieldsProps {
  agentName: string;
  provider: string;
  /** `null` forks the whole session. */
  pointQuote: string | null;
  /** The point is chosen on the message, so Change closes the dialog. */
  onChangePoint: () => void;
  disabled?: boolean;
}

/**
 * Agent (read-only: a fork is this agent; another agent is a Continue) and the
 * fork point — the whole session, or through the clicked message and its turn.
 */
export function SessionForkFields({
  agentName,
  provider,
  pointQuote,
  onChangePoint,
  disabled = false,
}: SessionForkFieldsProps) {
  return (
    <>
      <Field>
        <FieldLabel id="session-fork-agent-label">Agent</FieldLabel>
        <div
          aria-labelledby="session-fork-agent-label"
          className="flex min-h-input min-w-0 items-center gap-2 rounded-md border border-line-soft px-2.5 text-form-input text-fg"
          data-testid="session-fork-agent"
          role="group"
        >
          <OwnerAvatar name={agentName} ownerId={agentName} ownerKind="agent" size="sm" />
          <span className="truncate">{agentName}</span>
          {provider ? (
            <>
              <span aria-hidden="true" className="text-subtle">
                ·
              </span>
              <span className="truncate font-mono text-eyebrow text-faint">{provider}</span>
            </>
          ) : null}
        </div>
      </Field>

      <Field>
        <FieldLabel id="session-fork-point-label">Fork point</FieldLabel>
        <div
          aria-labelledby="session-fork-point-label"
          className="flex flex-col gap-1 text-form-input text-fg"
          data-testid="session-fork-point"
          role="group"
        >
          {pointQuote === null ? (
            <span>Whole session</span>
          ) : (
            <>
              <span>
                Through <q className="font-medium text-fg">{pointQuote}</q>
              </span>
              <Button
                // Inline beside the quote: no pill geometry; the underline is the non-color cue.
                className="h-auto self-start p-0 underline underline-offset-2"
                data-testid="session-fork-point-change"
                disabled={disabled}
                onClick={onChangePoint}
                size="xs"
                type="button"
                variant="link"
              >
                Change
              </Button>
            </>
          )}
        </div>
      </Field>
    </>
  );
}
