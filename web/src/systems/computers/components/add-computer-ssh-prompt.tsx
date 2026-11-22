import { useState } from "react";
import { Bot, Clock, Eye, LockKeyhole, ShieldAlert, ShieldCheck } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  CodeBlock,
  Disclosure,
  Field,
  FieldDescription,
  FieldLabel,
  Input,
  Pill,
} from "@compozy/ui";

import { KeyFingerprint } from "./key-fingerprint";

export interface AddComputerHostKeyCardProps {
  mode: "trust" | "changed";
  hostLabel: string;
  target: string;
  keyType: string;
  fingerprint: string;
  previousFingerprint?: string;
  verifyCommand?: string;
  onTrust?: () => void;
  onDecline?: () => void;
}

/**
 * Anchored inline host-key prompt directly beneath the active `Connecting over SSH` step:
 * - `trust`: left-ruled inline prompt with a single framed fingerprint block (no nested card)
 * - `changed`: blocked warning (`ssh_host_key_changed`) explaining rebuilt host vs interception
 */
export function AddComputerHostKeyCard({
  mode,
  hostLabel,
  target,
  keyType,
  fingerprint,
  previousFingerprint,
  verifyCommand = "ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub",
  onTrust,
  onDecline,
}: AddComputerHostKeyCardProps) {
  if (mode === "changed") {
    return (
      <div className="flex flex-col gap-3" data-slot="ssh-host-key-changed-card">
        <Alert variant="destructive">
          <ShieldAlert aria-hidden="true" />
          <AlertTitle>{hostLabel}'s key changed since you trusted it</AlertTitle>
          <AlertDescription className="flex flex-col gap-2">
            <p>
              Compozy blocked this connection to protect your credentials. This happens if{" "}
              <span className="font-mono text-mono-id text-fg">{target}</span> was rebuilt, or if
              another computer is answering at that address.
            </p>
          </AlertDescription>
        </Alert>

        <KeyFingerprint
          columns={3}
          label={`New ${keyType} fingerprint`}
          layout="grid"
          value={fingerprint}
        />

        {previousFingerprint ? (
          <div className="flex flex-wrap items-center justify-between gap-2 text-meta">
            <span className="text-subtle">Previously trusted</span>
            <KeyFingerprint copyable={false} layout="inline" value={previousFingerprint} />
          </div>
        ) : null}

        <Disclosure label="How to verify this host's fingerprint" variant="inline">
          <div className="flex flex-col gap-2">
            <p className="text-meta text-muted-foreground">
              Run this command directly on {hostLabel} and compare the output before removing the
              old entry from your local <span className="font-mono text-mono-id">known_hosts</span>:
            </p>
            <CodeBlock code={verifyCommand} copyable density="compact" language="bash" showPrompt />
          </div>
        </Disclosure>
      </div>
    );
  }

  return (
    <div
      className="flex flex-col gap-2.5 border-l-2 border-warning pl-3.5 py-0.5"
      data-slot="ssh-host-key-trust-card"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-small-body font-medium text-fg-strong">Trust this host?</span>
        <span className="font-mono text-mono-id text-muted-foreground">{target}</span>
      </div>
      <p className="text-meta text-muted-foreground">
        Compozy hasn't connected to <span className="font-medium text-fg">{hostLabel}</span> before.
        Confirm this fingerprint matches the computer you expect.
      </p>
      <KeyFingerprint
        columns={3}
        label={`${keyType} · SHA256 fingerprint`}
        layout="grid"
        value={fingerprint}
      />
      <div className="flex flex-wrap items-center justify-end gap-2 pt-0.5">
        <Button onClick={onDecline} size="sm" type="button" variant="outline">
          Decline
        </Button>
        <Button onClick={onTrust} size="sm" type="button">
          <ShieldCheck aria-hidden="true" />
          Trust and continue
        </Button>
      </div>
    </div>
  );
}

function formatCountdown(seconds: number): string {
  const clamped = Math.max(0, seconds);
  const mins = Math.floor(clamped / 60);
  const secs = clamped % 60;
  return `${mins}:${String(secs).padStart(2, "0")}`;
}

export interface AddComputerSecretCardProps {
  target: string;
  promptLabel?: string;
  countdownSeconds: number;
  /** Non-empty when an agent started setup and handed off the secret prompt to the operator. */
  agentSessionTitle?: string | null;
  errorMessage?: string | null;
  onSubmit?: (secret: string) => void;
  onRetry?: () => void;
  onCancel?: () => void;
}

/**
 * Anchored inline SSH password / 2FA prompt directly beneath the active
 * `Connecting over SSH` step (`_uiux.md` U2, `research-references` §2 Screen 03).
 */
export function AddComputerSecretCard({
  target,
  promptLabel = "SSH password or verification code",
  countdownSeconds,
  agentSessionTitle,
  errorMessage,
  onSubmit,
  onRetry,
  onCancel,
}: AddComputerSecretCardProps) {
  const [value, setValue] = useState("");
  const [revealed, setRevealed] = useState(false);
  const expired = countdownSeconds <= 0;

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    if (expired || !value.trim()) return;
    onSubmit?.(value);
  };

  return (
    <div
      className={
        expired
          ? "flex flex-col gap-2.5 border-l-2 border-warning pl-3.5 py-0.5"
          : "flex flex-col gap-2.5 border-l-2 border-line-strong pl-3.5 py-0.5"
      }
      data-slot="ssh-secret-prompt-card"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-small-body font-medium text-fg-strong">{promptLabel}</span>
        <Pill className="tabular-nums" size="xs" tone={expired ? "warning" : "neutral"}>
          <Clock aria-hidden="true" className="size-3" />
          {expired ? "Expired" : formatCountdown(countdownSeconds)}
        </Pill>
      </div>

      {agentSessionTitle ? (
        <div className="flex items-start gap-2 rounded-md border border-line-soft bg-sunken px-3 py-2 text-meta text-fg">
          <Bot aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />
          <span>
            Started by session <span className="font-medium">“{agentSessionTitle}”</span>. Agents
            cannot see or answer this prompt.
          </span>
        </div>
      ) : null}

      {expired ? (
        <div className="flex flex-col gap-2.5">
          <p className="text-small-body text-warning">
            This SSH prompt timed out before it was answered. Retry to reconnect to{" "}
            <span className="font-mono text-mono-id">{target}</span>.
          </p>
          <div className="flex items-center justify-end gap-2">
            <Button onClick={onCancel} size="sm" type="button" variant="outline">
              Cancel
            </Button>
            <Button onClick={onRetry} size="sm" type="button">
              Retry connection
            </Button>
          </div>
        </div>
      ) : (
        <form className="flex flex-col gap-2.5" onSubmit={handleSubmit}>
          <Field>
            <FieldLabel htmlFor="add-computer-ssh-secret">
              Password or code for{" "}
              <span className="font-mono text-mono-id text-muted-foreground">{target}</span>
            </FieldLabel>
            <div className="flex items-center gap-2">
              <div className="relative flex-1">
                <Input
                  autoComplete="off"
                  className="pr-9"
                  id="add-computer-ssh-secret"
                  onChange={event => setValue(event.target.value)}
                  placeholder="Enter password or one-time code"
                  type={revealed ? "text" : "password"}
                  value={value}
                />
                <Button
                  aria-label={revealed ? "Hide value" : "Show value"}
                  className="absolute top-1/2 right-1 -translate-y-1/2"
                  onClick={() => setRevealed(current => !current)}
                  size="icon-xs"
                  type="button"
                  variant="ghost"
                >
                  <Eye aria-hidden="true" className="size-3.5 text-subtle" />
                </Button>
              </div>
              <Button disabled={!value.trim()} size="sm" type="submit">
                Continue
              </Button>
            </div>
            <FieldDescription className="flex items-center gap-1.5">
              <LockKeyhole aria-hidden="true" className="size-3 shrink-0 text-faint" />
              <span>
                Entered only on your screen — sent to your local SSH process and never saved or
                shown to agents.
              </span>
            </FieldDescription>
          </Field>
          {errorMessage ? (
            <p className="text-meta text-destructive" role="alert">
              {errorMessage}
            </p>
          ) : null}
        </form>
      )}
    </div>
  );
}
