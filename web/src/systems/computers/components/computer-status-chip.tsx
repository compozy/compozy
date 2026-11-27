import type { ComponentProps } from "react";

import { cn, Pill, Spinner, type PillTone } from "@compozy/ui";

import type { ComputerStatus } from "../mocks/remote-computers-fixtures";

interface StatusPresentation {
  label: string;
  tone: PillTone;
  spinning?: boolean;
}

const COMPUTER_STATUS_PRESENTATION: Record<ComputerStatus, StatusPresentation> = {
  connected: { label: "Connected", tone: "success" },
  connecting: { label: "Connecting", tone: "neutral", spinning: true },
  offline: { label: "Offline", tone: "neutral" },
  needs_attention: { label: "Needs attention", tone: "warning" },
  incompatible: { label: "Incompatible", tone: "warning" },
  needs_pro: { label: "Needs Pro", tone: "brand" },
  disabled: { label: "Turned off", tone: "neutral" },
};

export interface ComputerStatusChipProps extends Omit<ComponentProps<"span">, "children"> {
  status: ComputerStatus;
  /** Screen-reader detail when the chip summarizes a failure or pending step. */
  detail?: string;
  /** Optional click handler (e.g. opening Computer details from a row). */
  onInspect?: () => void;
}

/**
 * Daemon-authoritative computer reachability status (`_uiux.md` §Signals):
 * plain toned dot or spinner beside text, never a boxed plate.
 */
export function ComputerStatusChip({
  status,
  detail,
  onInspect,
  className,
  ...props
}: ComputerStatusChipProps) {
  const presentation = COMPUTER_STATUS_PRESENTATION[status];
  const content = (
    <>
      {presentation.spinning ? (
        <Spinner aria-hidden="true" className="size-3 shrink-0" />
      ) : (
        <Pill.Dot />
      )}
      {presentation.label}
      {detail ? <span className="sr-only"> — {detail}</span> : null}
    </>
  );

  if (onInspect) {
    return (
      <Pill
        data-computer-status={status}
        form="plain"
        render={
          <button
            className={cn(
              "cursor-pointer rounded-sm transition-colors duration-base hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none",
              className
            )}
            onClick={onInspect}
            type="button"
          />
        }
        tone={presentation.tone}
        {...props}
      >
        {content}
      </Pill>
    );
  }

  return (
    <Pill
      className={className}
      data-computer-status={status}
      form="plain"
      tone={presentation.tone}
      {...props}
    >
      {content}
    </Pill>
  );
}
