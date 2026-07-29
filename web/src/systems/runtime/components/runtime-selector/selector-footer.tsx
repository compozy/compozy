import { Brain } from "lucide-react";
import { useId, type ReactNode } from "react";

import { type ReasoningEffort, type RuntimeSpeed } from "@/lib/api-contract";
import { Eyebrow } from "@compozy/ui";

import { ReasoningSlider } from "./reasoning-slider";
import { RuntimeSpeedSwitch } from "./speed-switch";
import { type RuntimeReasoningMode, type RuntimeReasoningState } from "./types";

export interface SelectorFooterProps {
  reasoning: RuntimeReasoningState;
  value: ReasoningEffort | "";
  modelName: string;
  onSelect: (effort: ReasoningEffort | "") => void;
  /**
   * Session-level ACP speed request (PR #267). The switch renders only when a
   * surface wires `onSpeedChange` — surfaces whose create contract has no
   * `speed` field never show a control they cannot send.
   */
  speed?: RuntimeSpeed;
  onSpeedChange?: (next: RuntimeSpeed) => void;
  speedDisabled?: boolean;
}

function FooterNote({
  mode,
  speedSwitch,
  children,
}: {
  mode: RuntimeReasoningMode;
  speedSwitch: ReactNode;
  children: ReactNode;
}) {
  return (
    <div
      className="flex h-10 shrink-0 items-center gap-2.5 border-t border-line-soft bg-canvas px-3"
      data-testid="runtime-selector-reasoning"
      data-reasoning-mode={mode}
    >
      <Brain aria-hidden="true" className="size-3.5 shrink-0 text-faint" />
      <span className="min-w-0 flex-1 truncate text-small-body text-subtle">{children}</span>
      {speedSwitch}
    </div>
  );
}

/**
 * Single-line popup footer (design ref: runtime-selector-variations.html,
 * variation A): the reasoning control and the ACP speed switch share one row.
 * With selectable levels the row is Eyebrow + compact slider + switch; without
 * them a one-line note keeps the row truthful about what the model supports.
 */
export function SelectorFooter({
  reasoning,
  value,
  modelName,
  onSelect,
  speed,
  onSpeedChange,
  speedDisabled = false,
}: SelectorFooterProps) {
  // Per-instance id so multiple mounted selectors never collide on one DOM id
  // (the label→slider `aria-labelledby` relationship must stay local).
  const labelId = useId();
  const speedSwitch = onSpeedChange ? (
    <RuntimeSpeedSwitch
      speed={speed ?? "normal"}
      disabled={speedDisabled}
      onSpeedChange={onSpeedChange}
    />
  ) : null;

  if (reasoning.mode === "no-model") {
    return (
      <FooterNote mode="no-model" speedSwitch={speedSwitch}>
        Select a model to set reasoning.
      </FooterNote>
    );
  }

  if (reasoning.mode === "none") {
    return (
      <FooterNote mode="none" speedSwitch={speedSwitch}>
        No reasoning effort for this model.
      </FooterNote>
    );
  }

  if (reasoning.mode === "supported-nolevels") {
    return (
      <FooterNote mode="supported-nolevels" speedSwitch={speedSwitch}>
        <b className="font-medium text-muted">Reasoning is on.</b> The provider decides.
      </FooterNote>
    );
  }

  return (
    <div
      className="flex h-11 shrink-0 items-center gap-2.5 border-t border-line-soft bg-canvas px-3"
      data-testid="runtime-selector-reasoning"
      data-reasoning-mode="levels"
    >
      <Eyebrow id={labelId} className="shrink-0 text-subtle">
        Reasoning
      </Eyebrow>
      <ReasoningSlider
        levels={reasoning.levels}
        value={value}
        defaultEffort={reasoning.defaultEffort}
        modelName={modelName}
        labelId={labelId}
        onSelect={onSelect}
      />
      {speedSwitch}
    </div>
  );
}
