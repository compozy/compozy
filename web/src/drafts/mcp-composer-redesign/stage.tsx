/**
 * A Claude session in the Sessions window with the real composer. `today` mounts the shipped
 * `MCPComposerControl`; the default mounts the redesigned popover from this draft.
 */
import type { SessionMCPResponse } from "@/systems/session/adapters/session-mcp-api";
import { MCPComposerControl } from "@/systems/session/components/mcp-composer-control";
import { sessionMCPSummaryFixture } from "@/systems/session/mocks/session-mcp-fixtures";

import { SessionsWindow } from "../always-on-agents-shared/session-frame";
import { GLOBAL_ROW } from "../always-on-agents-shared/sessions-fixtures";
import { INSTANCES } from "../always-on-agents-shared/fixtures";
import {
  AgentTurn,
  DraftComposer,
  TranscriptColumn,
  UserTurn,
} from "../always-on-agents-shared/transcript";

import { ComposerMCPPopover, type ComposerMCPPopoverProps } from "./composer-mcp-popover";

const RAIL_GROUP = {
  name: "compozy",
  count: 4,
  rows: ["s1"],
  threads: { s1: ["s4", "s2", "s9"] },
  instances: [INSTANCES.orchestratorCompozy, INSTANCES.triageCompozy],
};

export interface ComposerStageProps extends ComposerMCPPopoverProps {
  data: SessionMCPResponse | null;
  today?: boolean;
}

export function ComposerStage({ today = false, ...popover }: ComposerStageProps) {
  const data = popover.data;
  const codex = data?.runtime === "codex";
  const control =
    today && data ? (
      <MCPComposerControl
        summary={sessionMCPSummaryFixture(data)}
        data={data}
        onAction={() => undefined}
        onManage={() => undefined}
        defaultOpen
      />
    ) : (
      <ComposerMCPPopover {...popover} />
    );
  return (
    <SessionsWindow
      rail={{ groups: [{ ...RAIL_GROUP, currentRow: "s1" }], top: GLOBAL_ROW }}
      title="Weekly revenue report from the dashboard"
    >
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <TranscriptColumn>
          <UserTurn>Pull last week's revenue from the Branas dashboard and summarize it.</UserTurn>
          <AgentTurn>
            {
              "I can't reach the Branas dashboard yet: its MCP server needs you to sign in. Once you do, I'll pull the numbers."
            }
          </AgentTurn>
        </TranscriptColumn>
        <DraftComposer
          mcpControl={control}
          placeholder={codex ? "Message Codex" : "Message Claude"}
          runtime={
            codex
              ? { provider: "codex", model: "gpt-6-astra" }
              : { provider: "claude", model: "claude-opus-5-5" }
          }
        />
      </div>
    </SessionsWindow>
  );
}
