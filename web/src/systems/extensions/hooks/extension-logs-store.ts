import { createStoreLogic } from "@xstate/store";

export type ExtensionLogStreamStatus = "idle" | "connecting" | "live" | "reconnecting" | "paused";
type ExtensionLogConnectionStatus = Exclude<ExtensionLogStreamStatus, "paused">;

interface ExtensionLogsState {
  error: Error | null;
  follow: boolean;
  generation: number;
  retryToken: number;
  streamStatus: ExtensionLogConnectionStatus;
}

type ExtensionLogsEvents = {
  connecting: Record<never, never>;
  followChanged: { follow: boolean };
  retryRequested: Record<never, never>;
  streamFailed: { error?: Error; generation: number };
  streamOpened: { generation: number };
  streamSuspended: { generation: number };
};

export const extensionLogsLogic = createStoreLogic<ExtensionLogsState, ExtensionLogsEvents>({
  context: { error: null, follow: true, generation: 0, retryToken: 0, streamStatus: "idle" },
  on: {
    connecting: context =>
      context.follow && (context.streamStatus === "idle" || context.streamStatus === "reconnecting")
        ? {
            ...context,
            error: null,
            generation: context.generation + 1,
            streamStatus: "connecting",
          }
        : undefined,
    followChanged: (context, event) =>
      context.follow === event.follow
        ? undefined
        : {
            ...context,
            error: null,
            follow: event.follow,
            generation: context.generation + 1,
            streamStatus: "idle",
          },
    retryRequested: context =>
      context.follow
        ? {
            ...context,
            error: null,
            generation: context.generation + 1,
            retryToken: context.retryToken + 1,
            streamStatus: "idle",
          }
        : undefined,
    streamFailed: (context, event) =>
      event.generation === context.generation &&
      (context.streamStatus === "connecting" || context.streamStatus === "live")
        ? { ...context, error: event.error ?? context.error, streamStatus: "reconnecting" }
        : undefined,
    streamOpened: (context, event) =>
      event.generation === context.generation &&
      (context.streamStatus === "connecting" || context.streamStatus === "reconnecting")
        ? { ...context, error: null, streamStatus: "live" }
        : undefined,
    streamSuspended: (context, event) => {
      if (event.generation !== context.generation) return;
      return {
        ...context,
        error: null,
        generation: context.generation + 1,
        streamStatus: "idle",
      };
    },
  },
});
