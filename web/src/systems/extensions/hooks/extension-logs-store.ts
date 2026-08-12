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
};

export const extensionLogsLogic = createStoreLogic<ExtensionLogsState, ExtensionLogsEvents>({
  context: { entries: [], follow: true, streamStatus: "idle" },
  on: {
    connecting: context => ({ ...context, streamStatus: "connecting" }),
    entryReceived: (context, event) => ({
      ...context,
      entries: appendExtensionLogEntries(context.entries, [event.entry]),
      streamStatus: "live",
    }),
    followChanged: (context, event) => ({
      ...context,
      follow: event.follow,
      streamStatus: event.follow ? "idle" : "paused",
    }),
    streamFailed: context => ({ ...context, streamStatus: "reconnecting" }),
    streamOpened: context => ({ ...context, streamStatus: "live" }),
  },
});
