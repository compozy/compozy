/**
 * Ask an agent for something — the operator's path to a typed call.
 *
 * The contract editor is optional on purpose. A call without one still works;
 * one with a contract gets a checked answer, and the child gets exactly one
 * repair attempt if the first try misses. Both forms of `expect` the runtime
 * accepts are accepted here — an example shape or a full JSON Schema — because
 * they normalize to the same contract and pin the same digest.
 *
 * Validation happens twice, and the split matters: malformed JSON is caught
 * locally because the operator can see the caret; anything that parses is sent,
 * and the daemon's own `call_expect_invalid` is what decides whether the shape
 * is usable. The UI never second-guesses the validator.
 */
import { CornerUpRight } from "lucide-react";

import { ActionResultBanner, Button, Eyebrow, Panel, Textarea } from "@compozy/ui";

/**
 * Who the call goes to, and therefore what the operator is about to start.
 *
 * The difference is not cosmetic. A call aimed at a session revives that exact
 * helper with everything it already knows; a call aimed at a definition starts a
 * new one that knows nothing. Saying which is a fact the operator needs before
 * pressing the button, not after.
 */
export type AgentCallTarget =
  | { kind: "agent" }
  | { kind: "session"; sessionId: string; agentName: string };

export interface AgentCallComposeProps {
  agentName: string;
  target?: AgentCallTarget;
  /**
   * The prior call had a contract, and only its digest survived.
   *
   * A digest is a fingerprint, not a shape — the original cannot be recovered
   * from it. Sending without one would quietly downgrade a checked call to an
   * unchecked one, so Send stays blocked until the operator supplies a contract.
   */
  contractRequired?: boolean;
  prompt: string;
  onPromptChange: (next: string) => void;
  /** Raw editor text — not parsed JSON, so a half-typed contract survives a render. */
  expect: string;
  onExpectChange: (next: string) => void;
  onSubmit: () => void;
  pending?: boolean;
  /** The daemon's refusal, or a local parse failure. */
  failure?: { code: string; message: string } | null;
  /** The accepted call, until the operator edits again. */
  accepted?: { callId: string; childSessionId: string | null } | null;
  onOpenAcceptedCall?: (callId: string) => void;
  "data-testid"?: string;
}

export function AgentCallCompose({
  agentName,
  target = { kind: "agent" },
  contractRequired = false,
  prompt,
  onPromptChange,
  expect,
  onExpectChange,
  onSubmit,
  pending = false,
  failure = null,
  accepted = null,
  onOpenAcceptedCall,
  "data-testid": testId,
}: AgentCallComposeProps) {
  const empty = prompt.trim().length === 0;
  const contractMissing = contractRequired && expect.trim().length === 0;
  return (
    <Panel
      data-testid={testId}
      title={<Eyebrow>Call {agentName}</Eyebrow>}
      foot={
        <span className="flex w-full items-center gap-2">
          <span className="text-form text-muted">
            The helper rests up to an hour after finishing · no deadline unless you set one
          </span>
          <span className="flex-1" />
          <Button
            data-testid="agent-call-compose-submit"
            disabled={empty || contractMissing || pending}
            onClick={onSubmit}
            size="sm"
            type="button"
          >
            <CornerUpRight aria-hidden="true" />
            Call
          </Button>
        </span>
      }
    >
      {target.kind === "session" ? (
        <p className="mb-2 text-form text-muted" data-testid="agent-call-compose-target">
          Continues {target.agentName} in <span className="font-mono">{target.sessionId}</span>,
          with everything it already knows.
        </p>
      ) : (
        <p className="mb-2 text-form text-muted" data-testid="agent-call-compose-target">
          Starts a new {agentName} that knows nothing about the earlier work.
        </p>
      )}

      <Textarea
        aria-label={`What ${agentName} should do`}
        data-testid="agent-call-compose-prompt"
        disabled={pending}
        onChange={event => onPromptChange(event.target.value)}
        rows={2}
        value={prompt}
      />

      <div className="mt-2">
        <label className="mb-1 block text-form text-muted" htmlFor="agent-call-compose-expect">
          {contractRequired
            ? "What the answer must look like — an example or a JSON Schema"
            : "What the answer must look like (optional) — an example or a JSON Schema"}
        </label>
        {contractRequired ? (
          <p className="mb-1 text-form text-warning" data-testid="agent-call-compose-contract-note">
            The earlier call checked its answer against a contract, but only the contract&apos;s
            fingerprint was kept — the shape itself cannot be rebuilt from it. Write it again to
            keep the answer checked.
          </p>
        ) : null}
        <Textarea
          aria-label="Result contract"
          data-testid="agent-call-compose-expect"
          disabled={pending}
          id="agent-call-compose-expect"
          onChange={event => onExpectChange(event.target.value)}
          rows={3}
          value={expect}
          className="font-mono text-form"
        />
      </div>

      {failure ? (
        <ActionResultBanner
          className="mt-2"
          data-testid="agent-call-compose-error"
          description={<span className="font-mono text-form">{failure.code}</span>}
          title={failure.message}
          tone="danger"
        />
      ) : null}

      {accepted ? (
        <ActionResultBanner
          actions={
            onOpenAcceptedCall ? (
              <Button
                onClick={() => onOpenAcceptedCall(accepted.callId)}
                size="xs"
                type="button"
                variant="outline"
              >
                Open {accepted.callId}
              </Button>
            ) : undefined
          }
          className="mt-2"
          data-testid="agent-call-compose-accepted"
          description={
            accepted.childSessionId ? (
              <span className="font-mono text-form">
                {agentName} is working in {accepted.childSessionId}
              </span>
            ) : undefined
          }
          title="Call accepted."
          tone="success"
        />
      ) : null}
    </Panel>
  );
}
