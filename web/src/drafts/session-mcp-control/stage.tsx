/**
 * A Claude session in the Sessions window with the real composer, the MCP control in its action
 * row, and the transcript reload line (`_uiux.md` S3, S4) as the shipped `Marker` row renders it.
 */
import { AlertCircle, Plug } from "@compozy/ui/icons";

import { Marker } from "@compozy/ui";

import { SessionsWindow } from "../always-on-agents-shared/session-frame";
import { GLOBAL_ROW } from "../always-on-agents-shared/sessions-fixtures";
import { AgentTurn, DraftComposer, TranscriptColumn, UserTurn } from "../always-on-agents-shared/transcript";
import { INSTANCES } from "../always-on-agents-shared/fixtures";
import type { SessionMCPModel } from "./fixtures";
import { MCPComposerControl } from "./mcp-control";

const RAIL_GROUP = {
  name: "compozy",
  count: 4,
  rows: ["s1"],
  threads: { s1: ["s4", "s2", "s9"] },
  instances: [INSTANCES.orchestratorCompozy, INSTANCES.triageCompozy],
};

export type ReloadLine = "none" | "applied" | "replayed" | "failed";

export function ReloadMarker({
  line,
  runtime,
}: {
  line: Exclude<ReloadLine, "none">;
  runtime: string;
}) {
  if (line === "failed") {
    return (
      <Marker icon={<AlertCircle />} role="alert" tone="warning">
        <b>Couldn't start {runtime} with the new MCP servers</b> · your message wasn't sent
      </Marker>
    );
  }
  return (
    <Marker icon={<Plug />} role="status" tone="neutral">
      <b>MCP servers updated</b> ·{" "}
      {line === "replayed" ? "conversation continued from history" : "sentry added"}
    </Marker>
  );
}

export interface SessionStageProps {
  model: SessionMCPModel;
  open?: boolean;
  loading?: boolean;
  line?: ReloadLine;
}

interface ComposerRuntime {
  provider: string;
  model: string;
  label: string;
}

const CLAUDE_COMPOSER: ComposerRuntime = { provider: "claude", model: "claude-opus-5-5", label: "Claude" };

const COMPOSER_RUNTIME: Partial<Record<string, ComposerRuntime>> = {
  claude: CLAUDE_COMPOSER,
  codex: { provider: "codex", model: "gpt-6-astra", label: "Codex" },
};

export function SessionMCPStage({ model, open = true, loading = false, line = "none" }: SessionStageProps) {
  const composerRuntime = COMPOSER_RUNTIME[model.runtime.id] ?? CLAUDE_COMPOSER;
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
          {line !== "none" ? <ReloadMarker line={line} runtime={model.runtime.displayName} /> : null}
          {line === "applied" || line === "replayed" ? (
            <AgentTurn>
              {
                "Sentry is connected now. The checkout errors come from one release: `checkout@4.12.0` throws on an empty cart."
              }
            </AgentTurn>
          ) : null}
        </TranscriptColumn>
        <DraftComposer
          children={line === "failed" ? "Now check the checkout errors in Sentry, please." : undefined}
          environmentControl={<MCPComposerControl loading={loading} model={model} open={open} />}
          placeholder={`Message ${composerRuntime.label}`}
          runtime={composerRuntime}
        />
      </div>
    </SessionsWindow>
  );
}
