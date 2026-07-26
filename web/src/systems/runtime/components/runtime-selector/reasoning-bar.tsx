import { Brain } from "lucide-react";
import { useId, type ReactNode } from "react";

import { type ReasoningEffort } from "@/lib/api-contract";
import { Eyebrow } from "@agh/ui";

import { ReasoningSlider } from "./reasoning-slider";
import { type RuntimeReasoningState } from "./types";

export interface ReasoningBarProps {
  reasoning: RuntimeReasoningState;
  value: ReasoningEffort | "";
  providerName: string;
  modelName: string;
  onSelect: (effort: ReasoningEffort | "") => void;
}

function Note({ children }: { children: ReactNode }) {
  return (
    <div className="flex items-center gap-2.5 px-px py-0.5 text-small-body text-subtle">
      <Brain aria-hidden="true" className="size-4 shrink-0 text-faint" />
      <span>{children}</span>
    </div>
  );
}

export function ReasoningBar({
  reasoning,
  value,
  providerName,
  modelName,
  onSelect,
}: ReasoningBarProps) {
  // Per-instance id so multiple mounted selectors never collide on one DOM id
  // (the label→slider `aria-labelledby` relationship must stay local to this bar).
  const labelId = useId();
  if (reasoning.mode === "no-model") {
    return (
      <div
        className="shrink-0 border-t border-line-soft bg-canvas px-[13px] py-[11px]"
        data-testid="runtime-selector-reasoning"
        data-reasoning-mode="no-model"
      >
        <Note>Select a model to choose its reasoning effort.</Note>
      </div>
    );
  }

  if (reasoning.mode === "none") {
    return (
      <div
        className="shrink-0 border-t border-line-soft bg-canvas px-[13px] py-[11px]"
        data-testid="runtime-selector-reasoning"
        data-reasoning-mode="none"
      >
        <Note>This model doesn’t use reasoning effort; the provider handles it.</Note>
      </div>
    );
  }

  if (reasoning.mode === "supported-nolevels") {
    return (
      <div
        className="shrink-0 border-t border-line-soft bg-canvas px-[13px] py-[11px]"
        data-testid="runtime-selector-reasoning"
        data-reasoning-mode="supported-nolevels"
      >
        <Note>
          <b className="font-medium text-muted">Reasoning is on.</b> {providerName} doesn’t expose
          selectable effort for {modelName}.
        </Note>
      </div>
    );
  }

  const sourceLabel = reasoning.source === "acp" ? "ACP" : "CATALOG";

  return (
    <div
      className="shrink-0 border-t border-line-soft bg-canvas px-[13px] pt-[11px] pb-3.5"
      data-testid="runtime-selector-reasoning"
      data-reasoning-mode="levels"
    >
      <div className="mb-2.5 flex items-center gap-2">
        <Eyebrow id={labelId} className="shrink-0 text-subtle">
          Reasoning effort
        </Eyebrow>
        <span className="min-w-0 truncate font-mono text-badge text-faint">
          {providerName} · {modelName}
        </span>
        <span className="ml-auto shrink-0 rounded-xxs bg-badge-fill px-[5px] py-px font-mono text-micro font-semibold tracking-mono text-faint">
          {sourceLabel}
        </span>
      </div>
      <ReasoningSlider
        levels={reasoning.levels}
        value={value}
        defaultEffort={reasoning.defaultEffort}
        modelName={modelName}
        labelId={labelId}
        onSelect={onSelect}
      />
    </div>
  );
}
