import { cn } from "@agh/ui";

interface MonoTagProps extends React.ComponentProps<"span"> {
  children: React.ReactNode;
}

/**
 * A mono uppercase micro-badge (type / kind / status tag). `uppercase` lives on an
 * inner span so the outer mono typography never inlines the forbidden
 * `font-mono uppercase` tuple (compozy-design-system/no-inline-eyebrow, L-022).
 * Callers pass background / color / padding via `className`.
 */
export function MonoTag({ className, children, ...props }: MonoTagProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center font-mono text-badge font-semibold tracking-wide text-subtle",
        className
      )}
      {...props}
    >
      <span className="uppercase">{children}</span>
    </span>
  );
}
