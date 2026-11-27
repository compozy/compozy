import { cn, CopyIconButton } from "@compozy/ui";

export interface KeyFingerprintProps {
  /** Full fingerprint value, e.g. `SHA256:9aK2f8B1...`. Copied verbatim. */
  value: string;
  /** Optional prefix badge/label, e.g. `ED25519` or `Device key`. */
  label?: string;
  /**
   * `grid`: grouped monospace chunks for visual verification
   * (Host-key trust, Pending approvals, Phone approval wait).
   * `inline`: compact grouped mono readout for dense device rows.
   */
  layout?: "grid" | "inline";
  /** Grid column count when `layout="grid"`. Defaults to 3 when divisible by 3 and not 4, else 4. */
  columns?: 3 | 4;
  copyable?: boolean;
  className?: string;
}

function splitFingerprint(raw: string): { prefix: string | null; chunks: string[] } {
  const colonIdx = raw.indexOf(":");
  const prefix = colonIdx > 0 ? raw.slice(0, colonIdx) : null;
  const body = colonIdx > 0 ? raw.slice(colonIdx + 1) : raw;
  const clean = body.replace(/\s+/g, "");
  const chunks: string[] = [];
  for (let i = 0; i < clean.length; i += 4) {
    chunks.push(clean.slice(i, i + 4));
  }
  return { prefix, chunks };
}

/**
 * Grouped monospace cryptographic fingerprint with one-click copy
 * (`research-references` §4.1, inspired by Synara `RootFingerprint`).
 */
export function KeyFingerprint({
  value,
  label,
  layout = "inline",
  columns,
  copyable = true,
  className,
}: KeyFingerprintProps) {
  const { prefix, chunks } = splitFingerprint(value);

  if (layout === "grid") {
    return (
      <FingerprintGrid
        value={value}
        label={label}
        columns={columns}
        copyable={copyable}
        className={className}
        prefix={prefix}
        chunks={chunks}
      />
    );
  }

  const algorithm = prefix ? prefix + ":" : "";
  const compactDisplay =
    chunks.length > 4
      ? `${algorithm}${chunks[0]} ${chunks[1]} … ${chunks[chunks.length - 1]}`
      : `${algorithm}${chunks.join(" ")}`;

  return (
    <span
      className={cn(
        "inline-flex min-w-0 items-center gap-1 font-mono text-mono-id tracking-mono-id text-muted-foreground tabular-nums",
        className
      )}
      data-layout="inline"
      data-slot="key-fingerprint"
      title={value}
    >
      {label ? <span className="font-sans text-faint">{label}</span> : null}
      <span className="min-w-0 truncate select-all">{compactDisplay}</span>
      {copyable ? (
        <CopyIconButton
          className="size-5 text-subtle hover:text-fg"
          copiedLabel="Fingerprint copied"
          copiedToastLabel="Fingerprint copied"
          copyLabel="Copy key fingerprint"
          value={value}
        />
      ) : null}
    </span>
  );
}

function FingerprintGrid({
  value,
  label,
  columns,
  copyable,
  className,
  prefix,
  chunks,
}: Omit<KeyFingerprintProps, "layout"> & { prefix: string | null; chunks: string[] }) {
  const resolvedColumns: 3 | 4 =
    columns ?? (chunks.length % 3 === 0 && chunks.length % 4 !== 0 ? 3 : 4);
  return (
    <div
      className={cn(
        "flex flex-col gap-2 rounded-md border border-line bg-code-bg px-3 py-2.5",
        className
      )}
      data-columns={resolvedColumns}
      data-layout="grid"
      data-slot="key-fingerprint"
    >
      <div className="flex items-center justify-between gap-2">
        <span className="eyebrow text-subtle">
          {label ?? (prefix ? `${prefix} fingerprint` : "Key fingerprint")}
        </span>
        {copyable ? (
          <CopyIconButton
            copiedLabel="Fingerprint copied"
            copiedToastLabel="Fingerprint copied"
            copyLabel="Copy key fingerprint"
            value={value}
          />
        ) : null}
      </div>
      <div
        className={cn(
          "grid gap-x-3 gap-y-1 font-mono text-small-body tracking-wide text-fg tabular-nums select-all",
          resolvedColumns === 3 ? "grid-cols-3" : "grid-cols-4"
        )}
        title={value}
      >
        {chunks.map((chunk, idx) => (
          <span key={`${idx}-${chunk}`}>{chunk}</span>
        ))}
      </div>
    </div>
  );
}
