import { Copy, Globe, ListChecks, Plus, RefreshCcw, UserCheck, Zap } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import { Button, Empty, Eyebrow } from "@agh/ui";

import { getTaskTemplate, type TaskTemplate, type TaskTemplateId } from "../lib/task-templates";

export type TasksEmptyStateTone = "accent" | "info" | "warning" | "neutral";

interface TemplateSlot {
  id: TaskTemplateId;
  tone: TasksEmptyStateTone;
  icon: ReactNode;
}

/**
 * Four-card empty-state grid — `accent / info / warning /
 * neutral` tones only (the prior `violet / amber` palette is gone). Six
 * template definitions remain available to the editor; the empty state
 * surfaces a curated four that match the proposal reference.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §3
 */
const TEMPLATE_SLOTS: TemplateSlot[] = [
  { id: "one_shot", tone: "accent", icon: <Zap className="size-3" /> },
  { id: "recurring", tone: "info", icon: <RefreshCcw className="size-3" /> },
  { id: "human_in_loop", tone: "warning", icon: <UserCheck className="size-3" /> },
  { id: "remote_peer", tone: "neutral", icon: <Globe className="size-3" /> },
];

const TONE_CLASS: Record<TasksEmptyStateTone, string> = {
  accent: "bg-accent-tint text-accent",
  info: "bg-info-tint text-info",
  warning: "bg-warning-tint text-warning",
  neutral: "bg-canvas-tint text-muted",
};

export interface TasksEmptyStateProps {
  workspaceName?: string | null;
  onSelectTemplate: (templateId: TaskTemplateId) => void;
  onCopyCli?: () => void;
}

/**
 * Empty-state for the Tasks domain — Empty primitive head + 4-card template
 * grid with the new tone palette (accent / info / warning / neutral). Eyebrow
 * head.
 */
export function TasksEmptyState({
  workspaceName,
  onSelectTemplate,
  onCopyCli,
}: TasksEmptyStateProps) {
  const headline = workspaceName ? `No tasks yet in ${workspaceName}` : "No tasks yet";

  return (
    <div
      className="flex min-h-0 flex-1 overflow-y-auto px-6 pt-16 pb-10"
      data-testid="tasks-empty-state"
    >
      <div className="mx-auto flex w-full max-w-4xl flex-col gap-10">
        <Empty
          action={
            <>
              <Button
                data-testid="tasks-empty-cta-new"
                onClick={() => onSelectTemplate("one_shot")}
                size="lg"
                type="button"
              >
                <Plus className="size-3" />
                New task
              </Button>
              {onCopyCli ? (
                <Button
                  data-testid="tasks-empty-cta-cli"
                  onClick={onCopyCli}
                  size="lg"
                  type="button"
                  variant="neutral"
                >
                  <Copy className="size-3" />
                  <span className="font-mono text-eyebrow text-fg-strong">agh tasks new</span>
                </Button>
              ) : null}
            </>
          }
          description="Tasks are durable contracts of work. Each one can spawn runs across agents, respect dependencies, and live in workspace or global scope. Start from a template and keep the operational context visible as the queue grows."
          fill={false}
          icon={ListChecks}
          title={headline}
        />

        <section data-testid="tasks-empty-templates" className="flex flex-col gap-3">
          <header className="flex items-baseline justify-between gap-2">
            <Eyebrow data-testid="tasks-empty-templates-eyebrow">Start from a template</Eyebrow>
            <Eyebrow className="text-faint">{TEMPLATE_SLOTS.length} templates</Eyebrow>
          </header>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            {TEMPLATE_SLOTS.map(slot => (
              <TemplateCard
                key={slot.id}
                onSelect={() => onSelectTemplate(slot.id)}
                slot={slot}
                template={getTaskTemplate(slot.id)}
              />
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}

interface TemplateCardProps {
  template: TaskTemplate;
  slot: TemplateSlot;
  onSelect: () => void;
}

function TemplateCard({ template, slot, onSelect }: TemplateCardProps) {
  return (
    <button
      className={cn(
        "flex h-full flex-col gap-3 rounded-lg bg-canvas-soft p-4 text-left transition-colors duration-base ease-out",
        "hover:bg-elevated focus-visible:outline-none focus-visible:shadow-focus-inset"
      )}
      data-testid={`tasks-empty-template-${template.id}`}
      data-tone={slot.tone}
      onClick={onSelect}
      type="button"
    >
      <span
        aria-hidden="true"
        className={cn("flex size-6 items-center justify-center rounded", TONE_CLASS[slot.tone])}
      >
        {slot.icon}
      </span>
      <span className="text-(length:--text-section-head) font-medium tracking-section-head text-fg-strong">
        {template.label}
      </span>
      <p className="text-form-label leading-relaxed text-muted">{template.description}</p>
    </button>
  );
}
