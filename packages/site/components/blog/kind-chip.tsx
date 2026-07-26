import { cn } from "@agh/ui";

import { WIRE_KIND_DOT_CLASS, type WireKind } from "./kind-chip-data";

export type { WireKind } from "./kind-chip-data";

export interface BlogKindChipProps {
  kind: WireKind;
  label?: string;
  className?: string;
}

export function BlogKindChip({ kind, label, className }: BlogKindChipProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-chip border border-line px-1.5 py-px eyebrow font-semibold! text-subtle",
        className
      )}
    >
      <span className={cn("inline-block size-2 rounded-full", WIRE_KIND_DOT_CLASS[kind])} />
      {label ?? kind}
    </span>
  );
}
