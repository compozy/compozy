import { ChevronRight } from "lucide-react";
import { useId, type ReactNode } from "react";

import { cn, Collapsible, CollapsibleContent, CollapsibleTrigger, Pill } from "@agh/ui";

/**
 * The one Advanced fold per page (design system §08): chevron toggle, closed
 * by default, hosting the operator layer. Provenance chips are allowed only
 * inside this fold.
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
    <Collapsible
      {...(open !== undefined ? { open, onOpenChange } : { defaultOpen })}
      className={cn("overflow-hidden", !bare && "rounded-lg border border-line bg-canvas-soft")}
      data-testid={testId ?? "settings-advanced"}
    >
      <CollapsibleTrigger
        aria-controls={bodyId}
        className={cn(
          "group/advanced flex w-full items-center gap-2 py-3 text-left text-ws-name font-medium text-muted",
          bare ? "px-0" : "px-4",
          "transition-colors duration-base hover:text-fg",
          !bare && "hover:bg-row-hover",
          "focus-visible:outline-none focus-visible:shadow-focus-ring"
        )}
        data-testid="settings-advanced-toggle"
        type="button"
      >
        <ChevronRight
          aria-hidden="true"
          className="size-3.5 text-faint transition-transform duration-base group-data-panel-open/advanced:rotate-90"
        />
        {label}
      </CollapsibleTrigger>
      <CollapsibleContent
        className={cn(!bare && "border-t border-line-soft", padded && "flex flex-col gap-6 p-4")}
        id={bodyId}
        keepMounted
      >
        {children}
      </CollapsibleContent>
    </Collapsible>
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
