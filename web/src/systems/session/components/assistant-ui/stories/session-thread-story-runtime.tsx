import { type ComponentProps, type ReactNode, useEffect } from "react";
import type { StoryObj } from "@storybook/react-vite";
import { primarySessionFixture } from "@/systems/session/mocks";
import { sessionStore } from "@/systems/session/stores/session-store";
import { SessionThread } from "@/components/assistant-ui/session-thread";
import { SessionTranscriptThreadProvider } from "@/systems/session/lib/session-transcript-thread-context";
import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { HttpResponse } from "msw";
import { transcriptPayload } from "./session-thread-story-transcripts";

type Story = StoryObj<typeof SessionThread>;

export function GoalCommandErrorFixture({ children }: { children: ReactNode }) {
  useEffect(() => {
    sessionStore.trigger.goalCommandReported({
      sessionId: primarySessionFixture.id,
      result: {
        outcome: "error",
        reason_code: "goal_objective_required",
        replaced_run_id: null,
        snapshot: null,
      },
    });
    return () => {
      sessionStore.trigger.sessionInteractionRemoved({ sessionId: primarySessionFixture.id });
    };
  }, []);

  return children;
}

export const baseArgs = {
  sessionId: primarySessionFixture.id,
  agentName: primarySessionFixture.agent_name,
  canPrompt: true,
  onCancelPrompt: () => undefined,
};

export function renderWithTranscriptState(
  args: ComponentProps<typeof SessionThread>,
  state: {
    status: "pending" | "error" | "success";
    error?: Error | null;
  }
) {
  return (
    <SessionTranscriptThreadProvider
      messages={[]}
      status={state.status}
      error={state.error ?? null}
      retry={() => undefined}
    >
      <SessionThread {...args} />
    </SessionTranscriptThreadProvider>
  );
}

/** The session resource the status row reads while a turn runs (durable start, current tool). */
function runningSession(currentTool: string | undefined, agents = 0) {
  return {
    ...primarySessionFixture,
    activity: {
      current_tool: currentTool,
      elapsed_ms: 134_000,
      elapsed_seconds: 134,
      idle_seconds: 0,
      iteration_current: 1,
      iteration_max: 1,
      turn_id: "story-turn-quiet",
      turn_started_at: new Date(Date.now() - 134_000).toISOString(),
    },
    supervision: {
      quiet_warning: null,
      sources: [],
      work_signals: Array.from({ length: agents }, (_, index) => ({
        kind: "active_child" as const,
        since: new Date(Date.now() - 60_000).toISOString(),
        ref: `sess_child_${index + 1}`,
      })),
    },
  };
}

export function quietTimelineStory(
  transcript: Parameters<typeof transcriptPayload>[0],
  options: { running?: boolean; currentTool?: string; agents?: number } = {}
): Story {
  return {
    args: {
      ...baseArgs,
      isSessionRunning: options.running ?? false,
      statusSession: options.running
        ? runningSession(options.currentTool, options.agents)
        : primarySessionFixture,
    },
    parameters: {
      ...storybookMswParameters({
        session: [
          compozyApiMock.get(
            "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript",
            () => HttpResponse.json(transcriptPayload(transcript))
          ),
        ],
      }),
    },
  };
}
