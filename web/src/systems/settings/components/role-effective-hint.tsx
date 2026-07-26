export interface RoleEffectiveHintProps {
  /** Projected value; `null` means the runtime resolves it only at invocation. */
  effective: string | null;
  emptyLabel: string;
}

/**
 * Truthful effective-value hint. A `null` projection is stated as unresolved
 * rather than filled with a plausible default.
 */
export function RoleEffectiveHint({ effective, emptyLabel }: RoleEffectiveHintProps) {
  if (effective == null || effective.trim() === "") {
    return <span className="mt-0.5 block text-form-hint text-subtle">{emptyLabel}</span>;
  }
  return (
    <span className="mt-0.5 block text-form-hint text-subtle">
      Effective <span className="font-mono text-muted">{effective}</span>
    </span>
  );
}
