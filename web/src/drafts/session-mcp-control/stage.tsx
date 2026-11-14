/**
 * A Claude session in the Sessions window with the real composer, the production MCP control in
 * its action row, and the production transcript reload line (`_uiux.md` S3, S4), from fixtures.
 */
import { MCPComposerControl } from "@/systems/session/components/mcp-composer-control";
import { SessionMCPReloadMarker } from "@/systems/session/components/session-mcp-reload-marker";
import type { SessionMCPResponse } from "@/systems/session/adapters/session-mcp-api";
import type { SessionMCPReloadView } from "@/systems/session/lib/session-mcp-reload-event";
import { sessionMCPSummaryFixture } from "@/systems/session/mocks/session-mcp-fixtures";

import { SessionsWindow } from "../always-on-agents-shared/session-frame";
import { GLOBAL_ROW } from "../always-on-agents-shared/sessions-fixtures";
import {
  AgentTurn,
  DraftComposer,
  TranscriptColumn,
  UserTurn,
} from "../always-on-agents-shared/transcript";
import { INSTANCES } from "../always-on-agents-shared/fixtures";

const RAIL_GROUP = {
  name: "compozy",
  count: 4,
  rows: ["s1"],
  threads: { s1: ["s4", "s2", "s9"] },
  instances: [INSTANCES.orchestratorCompozy, INSTANCES.triageCompozy],
};

export interface SessionStageProps {
  data: SessionMCPResponse;
  open?: boolean;
  loading?: boolean;
  /** The last read failed: rows stay, marked out of date. */
  stale?: boolean;
  reload?: SessionMCPReloadView;
}

interface ComposerRuntime {
  provider: string;
  model: string;
  label: string;
}

const CLAUDE_COMPOSER: ComposerRuntime = {
  provider: "claude",
  model: "claude-opus-5-5",
  label: "Claude",
};

const COMPOSER_RUNTIME: Partial<Record<string, ComposerRuntime>> = {
  claude: CLAUDE_COMPOSER,
  codex: { provider: "codex", model: "gpt-6-astra", label: "Codex" },
};

export function SessionMCPStage({
  data,
  open = true,
  loading = false,
  stale = false,
  reload,
}: SessionStageProps) {
  const composerRuntime = COMPOSER_RUNTIME[data.runtime] ?? CLAUDE_COMPOSER;
  const continued = reload !== undefined && reload.outcome !== "failed";
  return (
    <SessionsWindow
      rail={{ groups: [{ ...RAIL_GROUP, currentRow: "s1" }], top: GLOBAL_ROW }}
      title="Triage the Sentry alerts from last night"
    >
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <TranscriptColumn>
          <UserTurn>Can you check what Sentry says about the checkout errors?</UserTurn>
          <AgentTurn>
            {
              "I don't have a Sentry tool in this session yet. If you add the Sentry MCP server, I can read the issues directly."
            }
          </AgentTurn>
          {reload ? (
            <SessionMCPReloadMarker reload={{ ...reload, runtime: data.runtime_display_name }} />
          ) : null}
          {continued ? (
            <AgentTurn>
              {
                "Sentry is connected now. The checkout errors come from one release: `checkout@4.12.0` throws on an empty cart."
              }
            </AgentTurn>
          ) : null}
        </TranscriptColumn>
        <DraftComposer
          children={
            reload?.outcome === "failed"
              ? "Now check the checkout errors in Sentry, please."
              : undefined
          }
          environmentControl={
            <MCPComposerControl
              summary={sessionMCPSummaryFixture(data)}
              data={loading ? null : data}
              loading={loading}
              failed={stale}
              onAction={() => undefined}
              onManage={() => undefined}
              defaultOpen={open}
            />
          }
          placeholder={`Message ${composerRuntime.label}`}
          runtime={composerRuntime}
        />
      </div>
    </SessionsWindow>
  );
}
