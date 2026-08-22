import { Copy, Globe, ListChecks, RefreshCcw, UserCheck, Zap } from "lucide-react";
import type { ReactNode } from "react";

import { CatalogEmptyPanel, CatalogEmptyState } from "@/components/catalog-empty-state";
import { Button, CatalogEmptyDisclosureRow, type CatalogEmptyTone, Pill } from "@compozy/ui";

import { getTaskTemplate, type TaskTemplate, type TaskTemplateId } from "../lib/task-templates";
import { emptyForScope } from "@/systems/profiles";

interface TemplateSlot {
  id: TaskTemplateId;
  tone: CatalogEmptyTone;
  icon: ReactNode;
}

/**
 * Curated zero-inventory templates — `accent / info / warning / neutral`
 * only. Six template definitions remain available to the editor.
 */
const TEMPLATE_SLOTS: TemplateSlot[] = [
  { id: "one_shot", tone: "accent", icon: <Zap className="size-3.5" /> },
  { id: "recurring", tone: "info", icon: <RefreshCcw className="size-3.5" /> },
  { id: "human_in_loop", tone: "warning", icon: <UserCheck className="size-3.5" /> },
  { id: "remote_peer", tone: "neutral", icon: <Globe className="size-3.5" /> },
];

export interface TasksEmptyStateProps {
  workspaceName?: string | null;
  /**
   * The profile the list is bounded by, or `null` under the aggregate where no
   * single profile bounds it. Never the create target.
   */
  profileName?: string | null;
  /** True while the aggregate is on, so the copy says so instead of naming one. */
  acrossProfiles?: boolean;
  onSelectTemplate: (templateId: TaskTemplateId) => void;
  onCopyCli?: () => void;
}

function templateFacts(template: TaskTemplate): string {
  const parts: string[] = [];
  if (template.defaults.priority) {
    parts.push(`priority ${template.defaults.priority}`);
  }
  if (template.defaults.max_attempts != null) {
    const attempts = template.defaults.max_attempts;
    parts.push(`${attempts} attempt${attempts === 1 ? "" : "s"}`);
  }
  if (template.defaults.approval_policy === "manual") {
    parts.push("approval manual");
  }
  if (template.defaults.draft) {
    parts.push("saves as draft");
  }
  parts.push(...(template.preview.facts ?? []));
  if (template.preview.enqueueOnSubmit) {
    parts.push("enqueues on submit");
  }
  return parts.join(" · ");
}

function templateDescription(template: TaskTemplate): string {
  return template.simpleDescription !== "" ? template.simpleDescription : template.description;
}

export function TasksEmptyState({
  workspaceName,
  profileName,
  acrossProfiles = false,
  onSelectTemplate,
  onCopyCli,
}: TasksEmptyStateProps) {
  // The profile axis is the narrower question, so it wins when it is known: an
  // operator in Marketing is being told this project is empty for Marketing, and
  // one looking at every profile is told the machine is empty — never that it is
  // empty in `default`, which is only where a new task would land.
  const headline =
    acrossProfiles || profileName
      ? emptyForScope("tasks", acrossProfiles ? null : (profileName ?? null))
      : workspaceName
        ? `No tasks yet in ${workspaceName}`
        : "No tasks yet";

  return (
    <CatalogEmptyState
      action={
        <Button
          data-testid="tasks-empty-cta-new"
          onClick={() => onSelectTemplate("blank")}
          size="sm"
          type="button"
          variant="neutral"
        >
          Blank task
        </Button>
      }
      data-testid="tasks-empty-state"
      icon={ListChecks}
      panel={
        <CatalogEmptyPanel
          aria-label="Task templates"
          count={TEMPLATE_SLOTS.length}
          data-testid="tasks-empty-templates"
          footer={
            <>
              <span>
                Or run{" "}
                <code className="rounded-mono-badge bg-badge-fill px-1.5 py-px font-mono text-mono-id text-muted">
                  compozy task create --scope workspace --title &quot;Your task&quot;
                </code>{" "}
                from a session.
              </span>
              {onCopyCli ? (
                <Button
                  data-testid="tasks-empty-cta-cli"
                  onClick={onCopyCli}
                  size="sm"
                  type="button"
                  variant="ghost"
                >
                  <Copy className="size-3" />
                  Copy
                </Button>
              ) : null}
            </>
          }
          label="Start from a template"
          note="Curated defaults — everything stays editable."
        >
          <ul>
            {TEMPLATE_SLOTS.map(slot => {
              const template = getTaskTemplate(slot.id);
              const badge = template.badges[0];
              return (
                <CatalogEmptyDisclosureRow
                  actions={
                    <Button
                      data-testid={`tasks-empty-template-${template.id}-use`}
                      onClick={() => onSelectTemplate(slot.id)}
                      size="sm"
                      type="button"
                      variant="neutral"
                    >
                      Use template
                    </Button>
                  }
                  description={templateDescription(template)}
                  detail={
                    <>
                      <div className="rounded-md border border-line-soft bg-input-fill px-3 py-2.5">
                        <p className="text-form-label leading-relaxed text-fg">
                          {template.description}
                          {template.preview.notice ? ` ${template.preview.notice}` : ""}
                        </p>
                      </div>
                      <p className="font-mono text-mono-id text-subtle tabular-nums">
                        {templateFacts(template)}
                      </p>
                    </>
                  }
                  icon={slot.icon}
                  data-testid={`tasks-empty-template-${template.id}`}
                  key={slot.id}
                  pill={
                    badge ? (
                      <Pill mono={false} size="sm" tone={badge.tone}>
                        {badge.label}
                      </Pill>
                    ) : null
                  }
                  title={template.label}
                  tone={slot.tone}
                />
              );
            })}
          </ul>
        </CatalogEmptyPanel>
      }
      support="A task is a durable contract of work — it spawns runs across agents."
      title={headline}
    />
  );
}
