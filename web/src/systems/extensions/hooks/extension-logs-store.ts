import { createStoreLogic } from "@xstate/store";

import { appendExtensionLogEntries } from "../lib/extension-log-stream";
import type { ExtensionLogEntry } from "../types";

export type ExtensionLogStreamStatus = "idle" | "connecting" | "live" | "reconnecting" | "paused";

interface ExtensionLogsState {
  entries: readonly ExtensionLogEntry[];
  follow: boolean;
  streamStatus: ExtensionLogStreamStatus;
}

type ExtensionLogsEvents = {
  connecting: Record<never, never>;
  entryReceived: { entry: ExtensionLogEntry };
  followChanged: { follow: boolean };
  streamFailed: Record<never, never>;
  streamOpened: Record<never, never>;
  streamSuspended: Record<never, never>;
};

export const extensionLogsLogic = createStoreLogic<ExtensionLogsState, ExtensionLogsEvents>({
  context: { entries: [], follow: true, streamStatus: "idle" },
  on: {
    connecting: context =>
      context.streamStatus === "idle" || context.streamStatus === "reconnecting"
        ? { ...context, streamStatus: "connecting" }
        : undefined,
    entryReceived: (context, event) => {
      if (context.streamStatus !== "connecting" && context.streamStatus !== "live") return;
      return {
        ...context,
        entries: appendExtensionLogEntries(context.entries, [event.entry]),
        streamStatus: "live",
      };
    },
    followChanged: (context, event) => ({
      ...context,
      follow: event.follow,
      streamStatus: event.follow ? "idle" : "paused",
    }),
    streamFailed: context =>
      context.streamStatus === "connecting" || context.streamStatus === "live"
        ? { ...context, streamStatus: "reconnecting" }
        : undefined,
    streamOpened: context =>
      context.streamStatus === "connecting" || context.streamStatus === "reconnecting"
        ? { ...context, streamStatus: "live" }
        : undefined,
    streamSuspended: context => {
      const streamStatus = context.follow ? "idle" : "paused";
      return context.streamStatus === streamStatus ? undefined : { ...context, streamStatus };
    },
  },
});
