import * as React from "react";

import { Pill } from "./pill";

const KIND_COLORS: Record<string, string> = {
  say: "var(--color-kind-say)",
  greet: "var(--color-kind-greet)",
  direct: "var(--color-kind-direct)",
  receipt: "var(--color-kind-receipt)",
  capability: "var(--color-kind-capability)",
  trace: "var(--color-kind-trace)",
  whois: "var(--color-kind-whois)",
};

function defaultKindColor(kind: string): string | undefined {
  return KIND_COLORS[kind.toLowerCase()];
}

export interface KindChipProps extends Omit<React.ComponentProps<"span">, "children"> {
  kind: string;
  /** Optional explicit label; defaults to `kind`. */
  label?: React.ReactNode;
  /** Optional explicit dot color; falls back to the protocol-kind registry. */
  dotColor?: string;
}

/**
 * Protocol kind marker — neutral tinted pill, sans label, leading colored dot
 * keyed off the protocol kind. Composes `Pill` + `Pill.Dot`. Unknown kinds
 * (platform names, event ids) render without a dot unless a `dotColor` is
 * supplied by the caller. Wrap the label in `<MonoId>` when the kind is a raw
 * identifier the reader needs to match character by character.
 */
export function KindChip({ kind, label, dotColor, className, ...props }: KindChipProps) {
  const color = dotColor ?? defaultKindColor(kind);
  return (
    <Pill
      size="xs"
      tone="neutral"
      data-slot="kind-chip"
      data-kind={kind}
      className={className}
      {...props}
    >
      {color ? <Pill.Dot color={color} /> : null}
      <span>{label ?? kind}</span>
    </Pill>
  );
}
