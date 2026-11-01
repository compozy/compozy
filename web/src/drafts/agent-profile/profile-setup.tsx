/** How the agent is set up: one quiet list, every row one click from the thing it describes. */
import { protoHref } from "@compozy/proto";
import {
  Bot,
  Brain,
  ChevronRight,
  Clock,
  Lock,
  Sparkles,
  Wrench,
  type LucideIcon,
} from "@compozy/ui/icons";

import { Panel, Section, cn } from "@compozy/ui";

import {
  HOME_RUN_ROW_CLASS,
  HOME_RUN_SUBTITLE_CLASS,
  HOME_RUN_TITLE_CLASS,
} from "@/systems/dashboard/components/home-zone-shape";

import type { DraftAgent } from "../stays-on-agents-shared/fixtures";

export type ProfilePanel = "model" | "tools";

function SetupRow({
  icon: Icon,
  title,
  summary,
  onOpen,
  locked,
}: {
  icon: LucideIcon;
  title: string;
  summary: string;
  onOpen: () => void;
  locked: boolean;
}) {
  const Trail = locked ? Lock : ChevronRight;
  return (
    <button
      className={cn(
        HOME_RUN_ROW_CLASS,
        "w-full text-left transition-colors duration-base hover:bg-surface-2 focus-visible:shadow-focus-inset focus-visible:outline-none"
      )}
      onClick={onOpen}
      type="button"
    >
      <span className="min-w-0">
        <span className={HOME_RUN_TITLE_CLASS}>
          <Icon aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
          <span className="truncate">{title}</span>
        </span>
        <span className={HOME_RUN_SUBTITLE_CLASS}>
          <span className="truncate pl-6">{summary}</span>
        </span>
      </span>
      <Trail aria-hidden="true" className="size-3.5 text-faint" />
    </button>
  );
}

function editor(state: string) {
  return () => window.location.assign(protoHref("agent-editors", state));
}

export function SetupPanel({
  agent,
  readOnly = false,
  onOpen,
}: {
  agent: DraftAgent;
  readOnly?: boolean;
  onOpen: (panel: ProfilePanel) => void;
}) {
  const staysOn = agent.kind === "stays_on";
  return (
    <Section
      label="How it's set up"
      right={
        readOnly ? (
          <span className="text-eyebrow text-muted-foreground">From {agent.owner}</span>
        ) : undefined
      }
    >
      <Panel bodyClassName="p-0" className="overflow-hidden">
        <div className="divide-y divide-line-soft">
          <SetupRow
            icon={Sparkles}
            locked={readOnly}
            onOpen={editor("what")}
            summary="Direct, and flags what blocks the work"
            title="How it behaves"
          />
          <SetupRow
            icon={Bot}
            locked={readOnly}
            onOpen={() => onOpen("model")}
            summary={agent.provider === "codex" ? "GPT 6 Astra" : "Claude Opus 5.5"}
            title="Model"
          />
          {staysOn ? (
            <SetupRow
              icon={Clock}
              locked={readOnly}
              onOpen={editor("checkins")}
              summary="Every 30 min, 9:00 to 19:00"
              title="Check-ins"
            />
          ) : null}
          {staysOn ? (
            <SetupRow
              icon={Brain}
              locked={readOnly}
              onOpen={editor("memory")}
              summary="14 notes"
              title="What it remembers"
            />
          ) : null}
          <SetupRow
            icon={Wrench}
            locked={readOnly}
            onOpen={() => onOpen("tools")}
            summary="12 tools in this project"
            title="What it can use"
          />
        </div>
      </Panel>
    </Section>
  );
}
