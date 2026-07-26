import {
  Calendar,
  Layers,
  ListChecks,
  Network,
  Sparkles,
  UserCheck,
  type LucideIcon,
} from "lucide-react";

import { Pill, RadioCard } from "@agh/ui";

import {
  SIMPLE_TASK_TEMPLATE_IDS,
  TASK_TEMPLATES,
  getTaskTemplate,
  type TaskTemplateId,
} from "../../lib/task-templates";

export type TaskFormMode = "simple" | "advanced";

interface TemplateCardsProps {
  mode: TaskFormMode;
  templateId: TaskTemplateId;
  onSelect: (id: TaskTemplateId) => void;
}

const TEMPLATE_ICONS: Record<TaskTemplateId, LucideIcon> = {
  one_shot: Sparkles,
  recurring: Calendar,
  epic: Layers,
  remote_peer: Network,
  human_in_loop: UserCheck,
  blank: ListChecks,
};

/**
 * Mode-aware template grid. Simple mode surfaces the three approachable
 * templates with their plain-language labels and no badge; Advanced mode lays
 * out every template with its precise label, description, and the first badge
 * rendered as a tone-matched Pill. Both lean on the shared accent-tint
 * canonical `RadioCard` affordance.
 */
export function TemplateCards({ mode, templateId, onSelect }: TemplateCardsProps) {
  if (mode === "simple") {
    return (
      <div aria-label="Task template" className="grid gap-2 sm:grid-cols-3" role="radiogroup">
        {SIMPLE_TASK_TEMPLATE_IDS.map(id => {
          const template = getTaskTemplate(id);
          return (
            <RadioCard
              data-testid={`task-template-${id}`}
              description={template.simpleDescription}
              icon={TEMPLATE_ICONS[id]}
              key={id}
              onSelect={() => onSelect(id)}
              selected={id === templateId}
              title={template.simpleLabel}
              titleClassName="whitespace-normal"
            />
          );
        })}
      </div>
    );
  }

  return (
    <div aria-label="Task template" className="grid gap-2 sm:grid-cols-2" role="radiogroup">
      {TASK_TEMPLATES.map(template => {
        const badge = template.badges[0];
        return (
          <RadioCard
            badge={
              badge ? (
                <Pill size="sm" tone={badge.tone}>
                  {badge.label}
                </Pill>
              ) : undefined
            }
            data-testid={`task-template-${template.id}`}
            description={template.description}
            icon={TEMPLATE_ICONS[template.id]}
            key={template.id}
            onSelect={() => onSelect(template.id)}
            selected={template.id === templateId}
            title={template.label}
            titleClassName="whitespace-normal"
          />
        );
      })}
    </div>
  );
}
