/** How the agent is set up: one quiet list, every row one click from the thing it describes. */
import { protoHref } from "@compozy/proto";
import { Bot, Brain, Clock, Sparkles, Wrench } from "@compozy/ui/icons";

import { Panel, Section } from "@compozy/ui";

import { SetupRow } from "@/systems/agent/components/agent-profile/agent-profile-setup";

import type { DraftAgent } from "../stays-on-agents-shared/fixtures";

export type ProfilePanel = "model" | "tools";

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
