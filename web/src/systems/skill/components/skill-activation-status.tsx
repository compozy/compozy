import { AlertCircle } from "lucide-react";
import * as React from "react";

import { cn, Pill, Section, type PillProps } from "@agh/ui";

import type { SkillActivationReasonPayload, SkillPayload } from "../types";

interface SkillActivationPillProps extends Omit<PillProps, "children" | "tone"> {
  active: boolean;
}

function SkillActivationPill({ active, ...props }: SkillActivationPillProps) {
  return (
    <Pill tone={active ? "success" : "warning"} {...props}>
      <Pill.Dot />
      {active ? "Active" : "Inactive"}
    </Pill>
  );
}

function formatMissing(missing: string[]): string {
  return missing.join(", ");
}

function formatRequiredContext(label: string, missing: string[]): string {
  const required = formatMissing(missing);
  return required === ""
    ? `${label} context unavailable`
    : `${label} context unavailable: ${required}`;
}

function formatMissingItems(singular: string, plural: string, missing: string[]): string {
  const label = missing.length === 1 ? singular : plural;
  return `${label}: ${formatMissing(missing)}`;
}

function formatSkillActivationReason(reason: SkillActivationReasonPayload): string {
  const missing = reason.missing ?? [];
  switch (reason.code) {
    case "platform_mismatch":
      return formatMissingItems("Required platform", "Required platforms", missing);
    case "environment_context_unavailable":
      return formatRequiredContext("Environment", missing);
    case "environment_mismatch":
      return formatMissingItems("Required environment", "Required environments", missing);
    case "tool_context_unavailable":
      return formatRequiredContext("Tool", missing);
    case "missing_tool":
      return formatMissingItems("Missing tool", "Missing tools", missing);
    case "capability_context_unavailable":
      return formatRequiredContext("Capability", missing);
    case "missing_capability":
      return formatMissingItems("Missing capability", "Missing capabilities", missing);
  }
}

interface SkillActivationReasonsProps extends React.ComponentProps<"ul"> {
  reasons: SkillActivationReasonPayload[];
  density?: "compact" | "full";
}

function SkillActivationReasons({
  reasons,
  density = "full",
  className,
  ...props
}: SkillActivationReasonsProps) {
  if (reasons.length === 0) return null;

  const visibleReasons = density === "compact" ? reasons.slice(0, 1) : reasons;
  const title = reasons.map(formatSkillActivationReason).join("; ");

  return (
    <ul
      aria-label="Inactive reasons"
      className={cn("min-w-0", density === "full" && "flex flex-col gap-1.5", className)}
      title={title}
      {...props}
    >
      {visibleReasons.map(reason => (
        <li
          className="flex min-w-0 items-start gap-1.5 text-small-body text-warning"
          key={`${reason.gate}-${reason.code}`}
        >
          <AlertCircle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0" />
          <span className={cn("min-w-0", density === "compact" ? "truncate" : "break-words")}>
            {formatSkillActivationReason(reason)}
            {density === "compact" && reasons.length > 1 ? ` · +${reasons.length - 1} more` : null}
          </span>
        </li>
      ))}
    </ul>
  );
}

function SkillActivationSection({ skill }: { skill: SkillPayload }) {
  return (
    <Section label="Activation">
      <div className="flex min-w-0 flex-col gap-2" data-testid="skill-activation-section">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <SkillActivationPill
            active={skill.activation.active}
            data-testid="skill-detail-activation-status"
          />
          <span className="text-small-body text-muted">
            {skill.activation.active
              ? "Activation gates are satisfied in this runtime context."
              : "Activation gates withhold this skill from agent prompts."}
          </span>
        </div>
        {!skill.activation.active ? (
          <SkillActivationReasons
            data-testid="skill-detail-activation-reasons"
            reasons={skill.activation.reasons ?? []}
          />
        ) : null}
      </div>
    </Section>
  );
}

export { SkillActivationPill, SkillActivationReasons, SkillActivationSection };
export type { SkillActivationPillProps, SkillActivationReasonsProps };
