import { useId, type ReactNode } from "react";

import { cn, Disclosure, Pill } from "@compozy/ui";

/**
 * The one Advanced fold per page (design system §08): the settings defaults
 * (testids, framed panel, kept-mounted body) over the generic `Disclosure`.
 * Provenance chips are allowed only inside this fold.
 */
export function SettingsAdvancedFold({
  children,
  defaultOpen = false,
  open,
  onOpenChange,
  padded = false,
  bare = false,
  label = "Advanced",
  "data-testid": testId,
}: {
  children: ReactNode;
  defaultOpen?: boolean;
  /** Controlled open state; when provided the fold ignores `defaultOpen`. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  /** Pad the body when it hosts whole groups instead of flush rows. */
  padded?: boolean;
  /**
   * Drop the panelbox frame so the fold can nest inside a surface that already
   * frames itself — nested frames are the anti-pattern this avoids.
   */
  bare?: boolean;
  /** Descriptive toggle label, e.g. "Advanced — limits". */
  label?: ReactNode;
  "data-testid"?: string;
}) {
  const bodyId = `settings-advanced-${useId().replace(/:/g, "")}`;

  return (
    <Disclosure
      {...(open !== undefined ? { open, onOpenChange } : { defaultOpen })}
      label={label}
      size="md"
      variant={bare ? "inline" : "framed"}
      keepMounted
      data-testid={testId ?? "settings-advanced"}
      className={bare ? "w-full" : undefined}
      triggerProps={{
        "aria-controls": bodyId,
        "data-testid": "settings-advanced-toggle",
        className: cn("text-ws-name", bare && "w-full px-0 py-3"),
      }}
      contentProps={{
        id: bodyId,
        className: cn(bare && "pt-0", padded && "flex flex-col gap-6 p-4"),
      }}
    >
      {children}
    </Disclosure>
  );
}

/** Mono provenance chip naming the real config key — Advanced-only. */
export function SettingsProvChip({ children }: { children: ReactNode }) {
  return (
    <Pill mono size="xs" tone="neutral">
      {children}
    </Pill>
  );
}
