import { useState } from "react";
import { Bot, Clock, Eye, LockKeyhole, ShieldAlert, ShieldCheck } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  CodeBlock,
  CopyIconButton,
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
  pinEntry?: string;
  keyType: string;
  fingerprint: string;
  previousFingerprint?: string;
  verifyCommand?: string;
  recoveryCommand?: string;
  onTrust?: () => void;
  onDecline?: () => void;
}

function splitFingerprintChunks(raw: string): string[] {
  const colonIdx = raw.indexOf(":");
  const body = colonIdx > 0 ? raw.slice(colonIdx + 1) : raw;
  const clean = body.replace(/\s+/g, "");
  const chunks: string[] = [];
  for (let i = 0; i < clean.length; i += 4) {
    chunks.push(clean.slice(i, i + 4));
  }
  return chunks;
}

function FingerprintPinCell({ label, value }: { label: string; value: string }) {
  const chunks = splitFingerprintChunks(value);
  return (
    <div className="flex flex-col gap-1.5 px-3 py-2.5">
      <div className="flex items-center justify-between gap-2">
        <span className="eyebrow text-subtle">{label}</span>
        <CopyIconButton
          className="size-5 text-subtle hover:text-fg"
          copiedLabel="Fingerprint copied"
          copiedToastLabel="Fingerprint copied"
          copyLabel={`Copy ${label.toLowerCase()} fingerprint`}
          value={value}
        />
      </div>
      <div
        className="grid grid-cols-3 gap-x-2.5 gap-y-0.5 font-mono text-mono-id tracking-wide text-fg tabular-nums select-all"
        title={value}
      >
        {chunks.map((chunk, idx) => (
          <span key={`${idx}-${chunk}`}>{chunk}</span>
        ))}
      </div>
    </div>
  );
}

/**
 * Anchored inline host-key prompt directly beneath the active `Connecting over SSH` step:
 * - `trust`: left-ruled inline prompt with a single framed fingerprint block (no nested card)
 * - `changed`: blocked warning (`ssh_host_key_changed`) with pinned vs observed fingerprints
 *   from the same pin entry and the exact recovery command (no Replace action, Q8)
 */
export function AddComputerHostKeyCard({
  mode,
  hostLabel,
  target,
  pinEntry,
  keyType,
  fingerprint,
  previousFingerprint,
  verifyCommand = "ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub",
  recoveryCommand,
  onTrust,
  onDecline,
}: AddComputerHostKeyCardProps) {
  if (mode === "changed") {
    return (
      <div className="flex flex-col gap-3" data-slot="ssh-host-key-changed-card">
        <Alert variant="destructive">
          <ShieldAlert aria-hidden="true" />
          <AlertTitle>{hostLabel}&apos;s key changed since you trusted it.</AlertTitle>
          <AlertDescription>
            Compozy blocked this connection to protect your credentials. Check the key directly on{" "}
            <span className="font-mono text-mono-id text-fg">{target}</span> before clearing the old
            pin.
          </AlertDescription>
        </Alert>

        <div className="overflow-hidden rounded-md border border-line bg-code-bg divide-y divide-line-soft">
          <div className="flex flex-wrap items-center justify-between gap-2 px-3 py-2 text-eyebrow">
            <span className="text-subtle">
              Pin entry{" "}
              <span className="font-mono text-mono-id text-fg">{pinEntry ?? target}</span>
            </span>
            <span className="font-mono text-micro text-faint">{keyType} · SHA256</span>
          </div>
          <div className="grid grid-cols-1 divide-y divide-line-soft sm:grid-cols-2 sm:divide-x sm:divide-y-0">
            {previousFingerprint ? (
              <FingerprintPinCell label="Pinned" value={previousFingerprint} />
            ) : null}
            <FingerprintPinCell label="Observed" value={fingerprint} />
          </div>
        </div>

        <div className="flex flex-col gap-2">
          <p className="text-meta text-muted-foreground">
            Check the host key on {hostLabel}, then remove the old pin and retry:
          </p>
          <CodeBlock code={verifyCommand} copyable density="compact" language="bash" showPrompt />
          {recoveryCommand ? (
            <CodeBlock
              code={recoveryCommand}
              copyable
              density="compact"
              language="bash"
              showPrompt
            />
          ) : null}
        </div>
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
                Entered only on your screen. Sent to your local SSH process and never saved or shown
                to agents.
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
